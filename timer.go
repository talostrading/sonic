package sonic

import (
	"time"

	"github.com/talostrading/sonic/internal"
)

type Timer struct {
	ioc *IO
	it  *internal.Timer
}

func NewTimer(ioc *IO) (*Timer, error) {
	it, err := internal.NewTimer(ioc.poller)
	if err != nil {
		return nil, err
	}

	return &Timer{
		ioc: ioc,
		it:  it,
	}, nil
}

func (t *Timer) Arm(dur time.Duration, onFire func()) error {
	err := t.it.Arm(dur, func() {
		delete(t.ioc.pendingTimers, t)
		onFire()
	})

	if err == nil {
		t.ioc.pendingTimers[t] = struct{}{}
	}

	return err
}

func (t *Timer) Disarm() error {
	err := t.it.Disarm()
	if err == nil {
		delete(t.ioc.pendingTimers, t)
	}
	return err
}

func (t *Timer) Close() error {
	err := t.it.Close()
	if err == nil {
		delete(t.ioc.pendingTimers, t)
	}
	return err
}
