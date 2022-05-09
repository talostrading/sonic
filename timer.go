package sonic

import (
	"time"

	"github.com/talostrading/sonic/internal"
)

type Timer struct {
	io *IO
	it *internal.Timer
	cb func()
}

func NewTimer(io *IO) (*Timer, error) {
	it, err := internal.NewTimer(io.poller)
	if err != nil {
		return nil, err
	}

	return &Timer{
		io: io,
		it: it,
	}, nil
}

func (t *Timer) Arm(dur time.Duration, onFire func()) error {
	return t.it.Arm(dur, onFire)
}

func (t *Timer) Disarm() error {
	return t.it.Disarm()
}
