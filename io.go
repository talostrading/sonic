package sonic

import (
	"io"

	"github.com/talostrading/sonic/internal"
)

type IO struct {
	poller *internal.Poller

	// inflightTimers prevents the PollData owned by the Timer to
	// be garbage collected while an async operation is in-flight,
	// in case the owning PollData goes out of scope
	inflightTimers map[*Timer]struct{}

	timeoutMs int
}

type pollOpts struct {
	Timeout int
}

func NewIO(timeout int) (*IO, error) {
	poller, err := internal.NewPoller()
	if err != nil {
		return nil, err
	}

	return &IO{
		poller:         poller,
		timeoutMs:      timeout,
		inflightTimers: make(map[*Timer]struct{}),
	}, nil
}

// Run runs the event processing loop
func (ioc *IO) Run() error {
	for {
		if err := ioc.RunOne(); err != nil && err != internal.ErrTimeout {
			return err
		}
	}
}

// TODO
func (ioc *IO) RunPending() error {
	return nil
}

// Poll runs the event processing loop to execute ready handlers
func (ioc *IO) Poll() error {
	for {
		if err := ioc.PollOne(); err != nil && err != internal.ErrTimeout {
			return err
		}
	}
}

// RunOne runs the event processing loop to execute at most one handler
// note: this blocks the calling coroutine in case timeoutMs is positive
func (ioc *IO) RunOne() error {
	if err := ioc.poller.Poll(ioc.timeoutMs); err != nil {
		if ioc.poller.Closed() {
			return io.EOF
		} else {
			return err
		}
	}
	return nil
}

// PollOne runs the event processing loop to execute one ready handler
// note: this will not block the calling goroutine
func (ioc *IO) PollOne() error {
	if err := ioc.poller.Poll(-1); err != nil {
		if ioc.poller.Closed() {
			return io.EOF
		} else {
			return err
		}
	}
	return nil
}

// TODO requires some mechanism to wake up the process as the handler is not bound by an fd
func (ioc *IO) Dispatch() error {
	return nil
}

func (ioc *IO) Close() error {
	return ioc.poller.Close()
}
