package sonic

import (
	"io"
	"runtime"
	"time"

	"github.com/talostrading/sonic/internal"
)

type IO struct {
	poller *internal.Poller

	// pending* prevents the PollData owned by an object to be garbage
	// collected while an async operation is in-flight on the object's file descriptor,
	// in case the object goes out of scope
	pendingReads, pendingWrites map[*internal.PollData]struct{}
	pendingTimers               map[*Timer]struct{}
}

func NewIO() (*IO, error) {
	poller, err := internal.NewPoller()
	if err != nil {
		return nil, err
	}

	return &IO{
		poller:        poller,
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

// Run runs the event processing loop
func (ioc *IO) Run() error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	for {
		if err := ioc.RunOne(); err != nil && err != internal.ErrTimeout {
			return err
		}
	}
}

// RunPending runs the event processing loop to execute all the pending handlers
func (ioc *IO) RunPending() error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	for n := ioc.poller.Pending(); n >= 0; n-- {
		if err := ioc.RunOne(); err != nil && err != internal.ErrTimeout {
			return err
		}
	}
	return nil
}

// RunOne runs the event processing loop to execute at most one handler
// note: this blocks the calling goroutine until one event is ready to process
func (ioc *IO) RunOne() error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	if err := ioc.poller.Poll(-1); err != nil {
		if ioc.poller.Closed() {
			return io.EOF
		} else {
			return err
		}
	}
	return nil
}

// RunOneFor runs the event processing loop for a specified duration to execute at
// most one handler.
// note: this blocks the calling goroutine until one event is ready to process
func (ioc *IO) RunOneFor(timeout time.Duration) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	if err := ioc.poller.Poll(int(timeout.Milliseconds())); err != nil {
		if ioc.poller.Closed() {
			return io.EOF
		} else {
			return err
		}
	}
	return nil
}

// Poll runs the event processing loop to execute ready handlers
// note: this will return immediately in case there is no event to process
func (ioc *IO) Poll() error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	for {
		if err := ioc.PollOne(); err != nil {
			return err
		}
	}
}

// PollOne runs the event processing loop to execute one ready handler
// note: this will return immediately in case there is no event to process
func (ioc *IO) PollOne() error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	if err := ioc.poller.Poll(0); err != nil {
		if ioc.poller.Closed() {
			return io.EOF
		} else {
			return err
		}
	}
	return nil
}

// Dispatch schedules the provided handler to be run immediately by the event
// processing loop in its own thread. It is safe to call this concurrently.
func (ioc *IO) Dispatch(handler func()) error {
	return ioc.poller.Dispatch(handler)
}

func (ioc *IO) Close() error {
	return ioc.poller.Close()
}
