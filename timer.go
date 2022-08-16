package sonic

import (
	"time"

	"github.com/talostrading/sonic/internal"
	"github.com/talostrading/sonic/sonicerrors"
)

type Timer struct {
	ioc   *IO
	it    *internal.Timer
	armed bool
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
	if t.armed {
		return sonicerrors.ErrCancelled
	}

	err := t.it.Arm(dur, func() {
		delete(t.ioc.pendingTimers, t)
		t.armed = false
		onFire()
	})

	if err == nil {
		t.ioc.pendingTimers[t] = struct{}{}
		t.armed = true
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
