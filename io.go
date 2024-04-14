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

// IO is the executor of all asynchronous operations and the way any object can schedule them. It runs fully in the
// calling goroutine.
//
// A goroutine must not have more than one IO. There might be multiple IOs in the same process, each within its own
// goroutine.
type IO struct {
	poller internal.Poller

	// The below structures keep a pointer to a PollData struct usually owned by an object capable of asynchronous
	// operations (essentially any object taking an IO* on construction). Keeping a PollData pointer keeps the owning
	// object in the GC's object graph while an asynchronous operation is in progress. This ensures PollData references
	// valid memory when an asynchronous operation completes and the object is already out of scope.
	pending struct {
		// The kernel allocates a process' descriptors from a fixed range that is [0, 1024) by default. Unprivileged
		// users can bump this range to [0, 4096). The below array should cover 99% of the cases and makes for cheap
		// PollData lookups.
		//
		// See https://github.com/torvalds/linux/blob/5939d45155bb405ab212ef82992a8695b35f6662/fs/file.c#L499 for how
		// file descriptors are bound by a fixed range whose upper limit is controlled through RLIMIT_NOFILE.
		static [4096]*internal.PollData

		// This map covers the 1%, the degenerate case. Any PollData whose file descriptor is greater than or equal to
		// 4096 goes here.
		dynamic map[*internal.PollData]struct{}
	}
	pendingTimers map[*Timer]struct{} // XXX: should be embedded into the above pending struct
}

func NewIO() (*IO, error) {
	poller, err := internal.NewPoller()
	if err != nil {
		return nil, err
	}

	return &IO{
		poller:        poller,
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

func (ioc *IO) Register(pd *internal.PollData) {
	if pd.Fd >= len(ioc.pending.static) {
		if ioc.pending.dynamic == nil {
			ioc.pending.dynamic = make(map[*internal.PollData]struct{})
		}
		ioc.pending.dynamic[pd] = struct{}{}
	} else {
		ioc.pending.static[pd.Fd] = pd
	}
}

func (ioc *IO) Deregister(pd *internal.PollData) {
	if pd.Fd >= len(ioc.pending.static) {
		delete(ioc.pending.dynamic, pd)
	} else {
		ioc.pending.static[pd.Fd] = nil
	}
}

// XXX SetRead/SetWrite should be on internal.PollData
func (ioc *IO) SetRead(slot *internal.PollData) error {
	return ioc.poller.SetRead(slot)
}

func (ioc *IO) SetWrite(slot *internal.PollData) error {
	return ioc.poller.SetWrite(slot)
}

// Run runs the event processing loop.
func (ioc *IO) Run() error {
	for {
		if err := ioc.RunOne(); err != nil && err != sonicerrors.ErrTimeout {
			return err
		}
	}
}

// RunPending runs the event processing loop to execute all the pending handlers. The function returns (and the event
// loop stops running) when there are no more operations to complete.
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

// RunOne runs the event processing loop to execute at most one handler.
//
// This call blocks the calling goroutine until an event occurs.
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

// RunOneFor runs the event processing loop for the given duration. The duration must not be lower than 1ms.
//
// This call blocks the calling goroutine until an event occurs.
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

// RunWarm runs the event loop in a combined busy-wait and yielding mode, meaning that if the current cycle does not
// process anything, the event-loop will busy-wait for at most `busyCycles` which we call the warm-state. After
// `busyCycles` of not processing anything, the event-loop is out of the warm-state and falls back to yielding with the
// provided timeout. If at any moment an event occurs and something is processed, the  event-loop transitions to its
// warm-state.
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
