package sonic

import (
	"fmt"
	"os"
	"runtime"
	"syscall"
	"time"

	"github.com/talostrading/sonic/internal"
	"github.com/talostrading/sonic/sonicerrors"
)

// IO executes scheduled asynchronous operations on a goroutine.
//
// A goroutine must not have more than one IO. Multiple goroutines, each with
// at most one IO can coexist in the same process.
type IO struct {
	poller internal.Poller

	// pending* prevents the PollData owned by an object to be garbage
	// collected while an async operation is in-flight on the object's file descriptor,
	// in case the object goes out of scope.
	// TODO get rid of map here. Only after subscription mechanism.
	// Per fd you can have a list of PollData
	pendingReads, pendingWrites map[*internal.PollData]struct{}
	pendingTimers               map[*Timer]struct{}
}

func NewIO() (*IO, error) {
	poller, err := internal.NewPoller()
	if err != nil {
		return nil, err
	}

	return &IO{
		poller: poller,

		// TODO pendingReads and pendingWrites should be merged in one. Also get rid of this map since everything
		// is on an fd
		pendingReads:  make(map[*internal.PollData]struct{}),
		pendingWrites: make(map[*internal.PollData]struct{}),

		pendingTimers: make(map[*Timer]struct{}),
	}, nil
}

func MustIO() *IO {
	ioc, err := NewIO()
	if err != nil {
		panic(err)
	}
	return ioc
}

func (ioc *IO) RegisterRead(pd *internal.PollData) {
	ioc.pendingReads[pd] = struct{}{}
}

func (ioc *IO) RegisterWrite(pd *internal.PollData) {
	ioc.pendingWrites[pd] = struct{}{}
}

func (ioc *IO) DeregisterRead(pd *internal.PollData) {
	delete(ioc.pendingReads, pd)
}

func (ioc *IO) DeregisterWrite(pd *internal.PollData) {
	delete(ioc.pendingWrites, pd)
}

func (ioc *IO) SetRead(fd int, slot *internal.PollData) error {
	return ioc.poller.SetRead(fd, slot)
}

func (ioc *IO) SetWrite(fd int, slot *internal.PollData) error {
	return ioc.poller.SetWrite(fd, slot)
}

// Run runs the event processing loop.
func (ioc *IO) Run() error {
	for {
		if err := ioc.RunOne(); err != nil && err != sonicerrors.ErrTimeout {
			return err
		}
	}
}

// RunPending runs the event processing loop to execute all the pending handlers.
//
// Subsequent handlers scheduled to run on a successful completion of the
// pending operation will not be executed.
func (ioc *IO) RunPending() error {
	for {
		if ioc.poller.Pending() <= 0 {
			break
		}

		if err := ioc.RunOne(); err != nil && err != sonicerrors.ErrTimeout {
			return err
		}
	}
	return nil
}

// RunOne runs the event processing loop to execute at most one handler
//
// This blocks the calling goroutine until one event is ready to process
func (ioc *IO) RunOne() (err error) {
	_, err = ioc.poll(-1)
	return
}

func checkTimeout(t time.Duration) error {
	if t < time.Millisecond {
		return fmt.Errorf("the provided duration's unit cannot be lower than a millisecond")
	}
	return nil
}

// RunOneFor runs the event processing loop for a specified duration to execute at
// most one handler. The provided duration should not be lower than a millisecond.
//
// This blocks the calling goroutine until one event is ready to process.
func (ioc *IO) RunOneFor(dur time.Duration) (err error) {
	if err := checkTimeout(dur); err != nil {
		return err
	}
	ms := int(dur.Milliseconds())
	_, err = ioc.poll(ms)
	return
}

const (
	WarmDefaultBusyCycles = 10
	WarmDefaultTimeout    = time.Millisecond
)

// RunWarm runs the reactor in a combined busy-wait and yielding mode, meaning that if the current cycle
// does not process anything, the reactor will busy-wait for at most `busyCycles` more which we call the warm-period.
// After `busyCycles` cycles of not processing anything, the reactor is out of the warm-period and falls back to
// yielding with the provided timeout. If at any moment an event occurs and something is processed, the reactor restarts
// its warm-period.
//
// RunWarm should be invoked with the above defaults: WarmDefaultBusyCycles and WarmDefaultTimeout.
func (ioc *IO) RunWarm(busyCycles int, timeout time.Duration) (err error) {
	if busyCycles <= 0 {
		return fmt.Errorf("busyCycles must be greater than 0")
	}
	if err = checkTimeout(timeout); err != nil {
		return err
	}

	var (
		t = int(timeout.Milliseconds())
		i = 0
		n int
	)
	for {
		if i < busyCycles {
			// We are still in the warm-period, we poll.
			n, err = ioc.poll(0)
		} else {
			// We are out of the warm-period, we yield for at most `t`.
			n, err = ioc.poll(t)
		}
		if err != nil && err != sonicerrors.ErrTimeout {
			return err
		}
		if n > 0 {
			// We processed something in this cycle, be it inside or outside the warm-period. We restart the warm-period
			i = 0
		} else {
			// We did not process anything in this cycle. If we are still in the warm period i.e. `i < busyCycles`,
			// we are going to poll in the next cycle. If we are outside the warm period i.e. `i >= busyCycles`,
			// we are going to yield in the next cycle.
			i++
		}
	}

	return nil
}

// Poll runs the event processing loop to execute ready handlers.
//
// This will return immediately in case there is no event to process.
func (ioc *IO) Poll() error {
	for {
		if _, err := ioc.PollOne(); err != nil {
			return err
		}
	}
}

// PollOne runs the event processing loop to execute one ready handler.
//
// This will return immediately in case there is no event to process.
func (ioc *IO) PollOne() (n int, err error) {
	return ioc.poll(0)
}

func (ioc *IO) poll(timeoutMs int) (int, error) {
	n, err := ioc.poller.Poll(timeoutMs)

	if err != nil {
		if err == syscall.EINTR {
			// TODO not sure about this one, and whether returning timeout here is ok.
			// need to look into syscall.EINTR again
			if timeoutMs >= 0 {
				return 0, sonicerrors.ErrTimeout
			}
			runtime.Gosched()
			return 0, nil
		}

		if err == sonicerrors.ErrTimeout {
			return 0, err
		}

		return 0, os.NewSyscallError(
			fmt.Sprintf("poll_wait timeout=%d", timeoutMs), err)
	}

	return n, nil
}

// Post schedules the provided handler to be run immediately by the event
// processing loop in its own thread.
//
// It is safe to call Post concurrently.
func (ioc *IO) Post(handler func()) error {
	return ioc.poller.Post(handler)
}

// Posted returns the number of handlers registered with Post.
//
// It is safe to call Posted concurrently.
func (ioc *IO) Posted() int {
	return ioc.poller.Posted()
}

func (ioc *IO) Pending() int64 {
	return ioc.poller.Pending()
}

func (ioc *IO) Close() error {
	return ioc.poller.Close()
}

func (ioc *IO) Closed() bool {
	return ioc.poller.Closed()
}
