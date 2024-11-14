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

// MaxCallbackDispatch is the maximum number of callbacks that can exist on a stack-frame when asynchronous operations
// can be completed immediately.
//
// This is the limit to the `IO.dispatched` counter.
const MaxCallbackDispatch int = 32

// IO is the executor of all asynchronous operations and the way any object can schedule them. It runs fully in the
// calling goroutine.
//
// A goroutine must not have more than one IO. There might be multiple IOs in the same process, each within its own
// goroutine.
type IO struct {
	poller internal.Poller

	// The below structures keep a pointer to a Slot struct usually owned by an object capable of asynchronous
	// operations (essentially any object taking an IO* on construction). Keeping a Slot pointer keeps the owning object
	// in the GC's object graph while an asynchronous operation is in progress. This ensures Slot references valid
	// memory when an asynchronous operation completes and the object is already out of scope.
	pending struct {
		// The kernel allocates a process' descriptors from a fixed range that is [0, 1024) by default. Unprivileged
		// users can bump this range to [0, 4096). The below array should cover 99% of the cases and makes for cheap
		// Slot lookups.
		//
		// See https://github.com/torvalds/linux/blob/5939d45155bb405ab212ef82992a8695b35f6662/fs/file.c#L499 for how
		// file descriptors are bound by a fixed range whose upper limit is controlled through RLIMIT_NOFILE.
		static [4096]*internal.Slot

		// This map covers the 1%, the degenerate case. Any Slot whose file descriptor is greater than or equal to 4096
		// goes here. This is lazily initialized.
		dynamic map[*internal.Slot]struct{}
	}
	pendingTimers map[*Timer]struct{} // XXX: should be embedded into the above pending struct

	// Tracks how many callbacks are on the current stack-frame. This prevents stack-overflows in cases where
	// asynchronous operations can be completed immediately.
	//
	// For example, an asynchronous read might be completed immediately. In that case, the callback is invoked which in
	// turn might call `AsyncRead` again. That asynchronous read might again be completed immediately and so on. In this
	// case, all subsequent read callbacks are placed on the same stack-frame. We count these callbacks with
	// `Dispatched`. If we hit `MaxCallbackDispatch`, then the stack-frame is popped - asynchronous reads are scheduled
	// to be completed on the next poll cycle, even if they can be completed immediately.
	//
	// This counter is shared amongst all asynchronous objects - they are responsible for updating it.
	Dispatched int
}

func NewIO() (*IO, error) {
	poller, err := internal.NewPoller()
	if err != nil {
		return nil, err
	}

	return &IO{
		poller:        poller,
		pendingTimers: make(map[*Timer]struct{}),
		Dispatched:    0,
	}, nil
}

func MustIO() *IO {
	ioc, err := NewIO()
	if err != nil {
		panic(err)
	}
	return ioc
}

func (ioc *IO) Register(slot *internal.Slot) {
	if slot.Fd >= len(ioc.pending.static) {
		if ioc.pending.dynamic == nil {
			ioc.pending.dynamic = make(map[*internal.Slot]struct{})
		}
		ioc.pending.dynamic[slot] = struct{}{}
	} else {
		ioc.pending.static[slot.Fd] = slot
	}
}

func (ioc *IO) Deregister(slot *internal.Slot) {
	if slot.Fd >= len(ioc.pending.static) {
		delete(ioc.pending.dynamic, slot)
	} else {
		ioc.pending.static[slot.Fd] = nil
	}
}

// SetRead tells the kernel to notify us when reads can be made on the provided IO slot. If successful, this call must
// be succeeded by Register(slot).
//
// It is safe to call this method multiple times.
func (ioc *IO) SetRead(slot *internal.Slot) error {
	return ioc.poller.SetRead(slot)
}

// UnsetRead tells the kernel to not notify us anymore when reads can be made on the provided IO slot. Since the
// underlying platform-specific poller already unsets a read before dispatching it, callers must only use this method
// they want to cancel a currently-scheduled read. For example, when an error occurs outside of an AsyncRead call and
// the underlying file descriptor must be closed. In that case, this call must be succeeded by Deregister(slot).
//
// It is safe to call this method multiple times.
func (ioc *IO) UnsetRead(slot *internal.Slot) error {
	return ioc.poller.DelRead(slot)
}

// Like SetRead but for writes.
func (ioc *IO) SetWrite(slot *internal.Slot) error {
	return ioc.poller.SetWrite(slot)
}

// Like UnsetRead but for writes.
func (ioc *IO) UnsetWrite(slot *internal.Slot) error {
	return ioc.poller.DelWrite(slot)
}

// UnsetRead and UnsetWrite in a single call.
func (ioc *IO) UnsetReadWrite(slot *internal.Slot) error {
	return ioc.poller.Del(slot)
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
// provided timeout. If at any moment an event occurs and something is processed, the event-loop transitions to its
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
			// We did not process anything in this cycle. If we are still in the warm period i.e. `i < busyCycles`, we
			// are going to poll in the next cycle. If we are outside the warm period i.e. `i >= busyCycles`, we are
			// going to yield in the next cycle.
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

// Post schedules the provided handler to be run immediately by the event processing loop in its own thread.
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

// Returns the current number of pending asynchronous operations.
func (ioc *IO) Pending() int64 {
	return ioc.poller.Pending()
}

func (ioc *IO) Close() error {
	return ioc.poller.Close()
}

func (ioc *IO) Closed() bool {
	return ioc.poller.Closed()
}
