//go:build linux

package internal

import (
	"os"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

var _ ITimer = &Timer{}

type Timer struct {
	fd     int
	poller Poller
	pd     PollData
	b      [8]byte
}

func NewTimer(p Poller) (*Timer, error) {
	fd, err := unix.TimerfdCreate(unix.CLOCK_REALTIME, unix.TFD_NONBLOCK)
	if err != nil {
		return nil, os.NewSyscallError("timerfd_create", err)
	}

	t := &Timer{
		fd:     fd,
		poller: p.(*poller),
	}
	t.pd.Fd = t.fd
	return t, nil
}

func (t *Timer) Set(dur time.Duration, cb func()) error {
	// first, make sure there's not another timer setup on the same fd
	if err := t.Unset(); err != nil {
		return err
	}

	timespec := unix.NsecToTimespec(dur.Nanoseconds())
	err := unix.TimerfdSettime(t.fd, 0, &unix.ItimerSpec{
		Interval: unix.Timespec{},
		Value:    timespec,
	}, nil)
	if err == nil {
		// TODO error checking here
		t.pd.Set(ReadEvent, func(error) {
			_, _ = syscall.Read(t.fd, t.b[:])
			cb()
		})
		err = t.poller.SetRead(t.fd, &t.pd)
	}

	return err
}

func (t *Timer) Unset() error {
	if t.pd.Flags&ReadFlags != ReadFlags {
		return nil
	}
	err := unix.TimerfdSettime(t.fd, 0, &unix.ItimerSpec{}, nil)
	if err == nil {
		err = t.poller.Del(t.fd, &t.pd)
	}
	return err
}

func (t *Timer) Close() error {
	_ = t.Unset()
	return syscall.Close(t.fd)
}
