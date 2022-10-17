//go:build darwin || netbsd || freebsd || openbsd || dragonfly

package internal

import (
	"math/rand"
	"syscall"
	"time"
)

var _ ITimer = &Timer{}

type Timer struct {
	fd     int
	poller *poller
	pd     PollData
}

func NewTimer(p Poller) (*Timer, error) {
	t := &Timer{
		fd:     rand.Int(), // TODO figure out something better
		poller: p.(*poller),
	}
	t.pd.Fd = t.fd
	return t, nil
}

func (t *Timer) Set(dur time.Duration, cb func()) error {
	// Make sure there's not another timer setup on the same fd.
	if err := t.Unset(); err != nil {
		return err
	}

	t.pd.Set(ReadEvent, func(_ error) { cb() })

	err := t.poller.set(t.fd, createEvent(
		syscall.EV_ADD|syscall.EV_ENABLE|syscall.EV_ONESHOT,
		syscall.EVFILT_TIMER,
		&t.pd,
		dur))

	if err == nil {
		t.poller.pending++
		t.pd.Flags |= ReadFlags
	}

	return err
}

func (t *Timer) Unset() error {
	if t.pd.Flags&ReadFlags != ReadFlags {
		return nil
	}
	err := t.poller.set(t.fd, createEvent(
		syscall.EV_DELETE|syscall.EV_DISABLE,
		syscall.EVFILT_TIMER, &t.pd, 0))
	if err == nil {
		t.poller.pending--
	}
	return err
}

func (t *Timer) Close() error {
	// There's no need to close the file descriptor as it's been chosen by
	// us and not returned by the kernel.
	return t.Unset()
}
