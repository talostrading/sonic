//go:build darwin || netbsd || freebsd || openbsd || dragonfly

package internal

import (
	"errors"
	"syscall"
	"time"
	"unsafe"
)

var ErrTimeout = errors.New("operation timed out")

type PollFlags int16

const (
	ReadFlags  = -PollFlags(syscall.EVFILT_READ)
	WriteFlags = -PollFlags(syscall.EVFILT_WRITE)
)

type Poller struct {
	kq int

	changelist []syscall.Kevent_t
	eventlist  []syscall.Kevent_t

	pd PollData

	pending int64

	closed uint8
}

func NewPoller() (*Poller, error) {
	kq, err := syscall.Kqueue()
	if err != nil {
		return nil, err
	}

	_, err = syscall.Kevent(kq, []syscall.Kevent_t{{
		Ident:  uint64(kq),
		Filter: syscall.EVFILT_USER,
		Flags:  syscall.EV_ADD | syscall.EV_CLEAR,
	}}, nil, nil) // listen to user events by default
	if err != nil {
		syscall.Close(kq)
		return nil, err
	}

	p := &Poller{
		kq:         kq,
		changelist: make([]syscall.Kevent_t, 0, 8),
		eventlist:  make([]syscall.Kevent_t, 128),
	}

	return p, nil
}

func (p *Poller) Poll(timeoutMs int) error {
	var timeout *syscall.Timespec
	if timeoutMs >= 0 { // 0 does a poll
		ts := syscall.NsecToTimespec(int64(timeoutMs) * 1e6)
		timeout = &ts
	}

	changelist := p.changelist
	p.changelist = p.changelist[:0]

	n, err := syscall.Kevent(p.kq, changelist, p.eventlist, timeout)
	if err != nil {
		return err
	}

	if n == 0 && timeoutMs >= 0 {
		return ErrTimeout
	}

	for i := 0; i < n; i++ {
		event := &p.eventlist[i]

		flags := -PollFlags(event.Filter)
		pd := (*PollData)(unsafe.Pointer(event.Udata))

		if flags&pd.Flags&ReadFlags == ReadFlags {
			p.pending--
			pd.Cbs[ReadEvent](nil)
		}

		if flags&pd.Flags&WriteFlags == WriteFlags {
			p.pending--
			pd.Cbs[WriteEvent](nil)
		}
	}

	return nil
}

func (p *Poller) SetRead(fd int, pd *PollData) error {
	pdflags := &pd.Flags
	if *pdflags&ReadFlags != ReadFlags {
		p.pending++
		*pdflags |= ReadFlags
		return p.set(fd, createEvent(syscall.EV_ADD|syscall.EV_ONESHOT, ReadFlags, pd))
	}
	return nil
}

func (p *Poller) SetWrite(fd int, pd *PollData) error {
	pdflags := &pd.Flags
	if *pdflags&WriteFlags != WriteFlags {
		p.pending++
		*pdflags |= WriteFlags
		return p.set(fd, createEvent(syscall.EV_ADD|syscall.EV_ONESHOT, WriteFlags, pd))
	}
}

func (p *Poller) DelRead(fd int, pd *PollData) error {
	pdflags := &pd.Flags
	if *pdflags&ReadFlags == ReadFlags {
		p.pending--
		*pdflags ^= ReadFlags
		return p.set(createEvent(syscall.EV_DELETE, ReadFlags, pd))
	}
}

func (p *Poller) DelWrite(fd int, pd *PollData) error {
	pdflags := &pd.Flags
	if *pdflags&WriteFlags == WriteFlags {
		p.pending--
		*pdflags ^= WriteFlags
		return p.set(createEvent(syscall.EV_DELETE, WriteFlags, pd))
	}
}

func (p *Poller) Del(fd int, pd *PollData) error {
	err := p.DelRead(fd, pd)
	if err == nil {
		err = p.DelWrite(fd, pd)
	}

	return err
}

func (p *PollData) set(fd int, ev syscall.Kevent_t) {
	ev.Ident = uint64(fd)
	p.changelist = append(p.changelist, ev)
}

func createEvent(flags uint16, filter PollFlags, pd *PollData, tickerTimeout time.Duration) syscall.Kevent_t {
	ev := syscall.Kevent_t{
		Flags:  flags,
		Filter: int16(-filter),
	}

	if pd != nil {
		ev.Udata = (*byte)(unsafe.Pointer(pd))
	}

	return ev
}
