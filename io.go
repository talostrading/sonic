package sonic

import "github.com/talostrading/sonic/internal"

type IO struct {
	poller    *internal.Poller
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
		poller:    poller,
		timeoutMs: timeout,
	}, nil
}

// Run runs the event processing loop
func (io *IO) Run() error {
	for {
		if err := io.RunOne(); err != nil && err != internal.ErrTimeout {
			return err
		}
	}
}

// TODO
func (io *IO) RunPending() error {
	return nil
}

// Poll runs the event processing loop to execute ready handlers
func (io *IO) Poll() error {
	for {
		if err := io.PollOne(); err != nil && err != internal.ErrTimeout {
			return err
		}
	}
}

// RunOne runs the event processing loop to execute at most one handler
// note: this blocks the calling coroutine in case timeoutMs is positive
func (io *IO) RunOne() error {
	return io.poller.Poll(io.timeoutMs)
}

// PollOne runs the event processing loop to execute one ready handler
// note: this will not block the calling goroutine
func (io *IO) PollOne() error {
	return io.poller.Poll(0)
}

// TODO requires some mechanism to wake up the process as the handler is not bound by an fd
func (io *IO) Dispatch() error {
	return nil
}
