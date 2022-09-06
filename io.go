package sonic

import (
	"fmt"
	"io"
	"os"
	"runtime"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/talostrading/sonic/internal"
	"github.com/talostrading/sonic/sonicerrors"
)

type IO struct {
	poller internal.Poller

	// pending* prevents the PollData owned by an object to be garbage
	// collected while an async operation is in-flight on the object's file descriptor,
	// in case the object goes out of scope.
	pendingReads, pendingWrites map[*internal.PollData]struct{}
	pendingTimers               map[*Timer]struct{}

	closed uint32
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

// RunOneFor runs the event processing loop for a specified duration to execute at
// most one handler. The provided duration should not be lower than a millisecond.
//
// This blocks the calling goroutine until one event is ready to process
func (ioc *IO) RunOneFor(dur time.Duration) (err error) {
	ms := int(dur.Milliseconds())
	_, err = ioc.poll(ms)
	return
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

func (ioc *IO) poll(timeoutMs int) (n int, err error) {
	n, err = ioc.poller.Poll(timeoutMs)

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

		return 0, os.NewSyscallError(fmt.Sprintf("poll_wait timeout=%d", timeoutMs), err)
	}

	return
}

// Post schedules the provided handler to be run immediately by the event
// processing loop in its own thread.
//
// It is safe to call Post concurrently.
func (ioc *IO) Post(handler func()) error {
	return ioc.poller.Post(handler)
}

func (ioc *IO) Pending() int64 {
	return ioc.poller.Pending()
}

func (ioc *IO) Close() error {
	if !atomic.CompareAndSwapUint32(&ioc.closed, 0, 1) {
		return io.EOF
	}

	return ioc.poller.Close()
}
