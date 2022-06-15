//go:build darwin || netbsd || freebsd || openbsd || dragonfly

package internal

import (
	"math/rand"
	"syscall"
	"time"
)

type Timer struct {
	fd     int
	poller *Poller
	pd     PollData
}

func NewTimer(poller *Poller) (*Timer, error) {
	t := &Timer{
		fd:     rand.Int(), // TODO figure out something better
		poller: poller,
	}
	t.pd.Fd = t.fd
	return t, nil
}

func (t *Timer) Arm(dur time.Duration, onFire func()) error {
	// first, make sure there's not another timer setup on the same fd
	if err := t.Disarm(); err != nil {
		return err
	}
	t.pd.Set(ReadEvent, func(_ error) { onFire() })

	err := t.poller.set(t.fd, createEvent(syscall.EV_ADD|syscall.EV_ENABLE|syscall.EV_ONESHOT, syscall.EVFILT_TIMER, &t.pd, dur))
	if err == nil {
		t.poller.pending++
		t.pd.Flags |= ReadFlags
	}
	return nil
}

func (t *Timer) Disarm() error {
	if t.pd.Flags&ReadFlags != ReadFlags {
		return nil
	}
	err := t.poller.set(t.fd, createEvent(syscall.EV_DELETE|syscall.EV_DISABLE, syscall.EVFILT_TIMER, &t.pd, 0))
	if err == nil {
		t.poller.pending--
	}
	return err
}

func (t *Timer) Close() error {
	return t.Disarm()
}
