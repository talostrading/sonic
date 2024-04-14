//go:build darwin || netbsd || freebsd || openbsd || dragonfly

package internal

import (
	"math/rand" //#nosec G404 -- randint is used as a timer file descriptor
	"syscall"
	"time"
)

var _ ITimer = &Timer{}

type Timer struct {
	fd     int
	poller *poller
	slot   Slot
}

func NewTimer(p Poller) (*Timer, error) {
	t := &Timer{
		/* #nosec G404 -- randint is used as a timer file descriptor */
		fd:     rand.Int(), // TODO figure out something better
		poller: p.(*poller),
	}
	t.slot.Fd = t.fd
	return t, nil
}

func (t *Timer) Set(dur time.Duration, cb func()) error {
	// Make sure there's not another timer setup on the same fd.
	if err := t.Unset(); err != nil {
		return err
	}

	t.slot.Set(ReadEvent, func(_ error) { cb() })

	err := t.poller.set(t.fd, createEvent(
		syscall.EV_ADD|syscall.EV_ENABLE|syscall.EV_ONESHOT,
		syscall.EVFILT_TIMER,
		&t.slot,
		dur))

	if err == nil {
		t.poller.pending++
		t.slot.Events |= PollerReadEvent
	}

	return err
}

func (t *Timer) Unset() error {
	if t.slot.Events&PollerReadEvent != PollerReadEvent {
		return nil
	}

	// We should actually be calling poller.Del but that besides EV_DELETE we also need EV_DISABLE for a timer,
	// so we delete it here.
	t.slot.Events ^= PollerReadEvent
	t.poller.pending--

	return t.poller.set(t.fd, createEvent(
		syscall.EV_DELETE|syscall.EV_DISABLE,
		syscall.EVFILT_TIMER, &t.slot, 0))
}

func (t *Timer) Close() error {
	// There's no need to close the file descriptor as it's been chosen by
	// us and not returned by the kernel.
	return t.Unset()
}
