//go:build darwin || netbsd || freebsd || openbsd || dragonfly

package internal

import (
	"io"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"
)

type PollFlags int16

const (
	ReadFlags  = -PollFlags(syscall.EVFILT_READ)
	WriteFlags = -PollFlags(syscall.EVFILT_WRITE)
)

type Poller struct {
	fd int

	changelist []syscall.Kevent_t
	eventlist  []syscall.Kevent_t

	lck      sync.Mutex
	handlers []func()

	pipe *Pipe

	// pending is the number of pending handlers in the poller needs to execute
	pending int64

	closed uint32

	oneByte [1]byte
}

func NewPoller() (*Poller, error) {
	pipe, err := NewPipe()
	if err != nil {
		return nil, err
	}

	if err := pipe.SetReadNonblock(); err != nil {
		return nil, err
	}

	if err := pipe.SetWriteNonblock(); err != nil {
		return nil, err
	}

	kq, err := syscall.Kqueue()
	if err != nil {
		return nil, err
	}

	_, err = syscall.Kevent(kq, []syscall.Kevent_t{{
		Ident:  uint64(kq),
		Filter: syscall.EVFILT_USER,
		Flags:  syscall.EV_ADD | syscall.EV_CLEAR,
	}}, nil, nil)
	if err != nil {
		syscall.Close(kq)
		return nil, err
	}

	p := &Poller{
		pipe:       pipe,
		fd:         kq,
		changelist: make([]syscall.Kevent_t, 0, 8),
		eventlist:  make([]syscall.Kevent_t, 128),
	}

	err = p.setRead(p.pipe.ReadFd(), syscall.EV_ADD, &p.pipe.pd)
	if err != nil {
		p.pipe.Close()
		syscall.Close(kq)
		return nil, err
	}
	p.pending-- // ignore the pipe read

	return p, nil
}

func (p *Poller) Pending() int64 {
	return p.pending
}

func (p *Poller) Close() error {
	if !atomic.CompareAndSwapUint32(&p.closed, 0, 1) {
		return io.EOF
	}

	p.changelist = nil
	p.eventlist = nil
	p.pending = 0

	p.pipe.Close()

	return syscall.Close(p.fd)
}

func (p *Poller) Closed() bool {
	return atomic.LoadUint32(&p.closed) == 1
}

func (p *Poller) Dispatch(handler func()) error {
	p.lck.Lock()
	p.handlers = append(p.handlers, handler)
	p.lck.Unlock()

	p.pending++

	// notify the operating system that the event processing loop
	// should run the provided handler
	_, err := p.pipe.Write([]byte{0})
	return err
}

func (p *Poller) dispatch() {
	for {
		_, err := p.pipe.Read(p.oneByte[:])
		if err != nil {
			break
		}
	}

	// TODO check pending
	p.lck.Lock()
	for _, handler := range p.handlers {
		handler()
		p.pending--
	}
	p.handlers = p.handlers[:0]
	p.lck.Unlock()
}

func (p *Poller) Poll(timeoutMs int) error {
	// 0 polls
	// -1 waits indefinitely // TODO standardize timeouts
	var timeout *syscall.Timespec
	if timeoutMs >= 0 {
		ts := syscall.NsecToTimespec(int64(timeoutMs) * 1e6)
		timeout = &ts
	}

	changelist := p.changelist
	p.changelist = p.changelist[:0]

	n, err := syscall.Kevent(p.fd, changelist, p.eventlist, timeout)
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

		if pd.Fd == p.pipe.ReadFd() {
			p.dispatch()
			continue
		}

		if flags&pd.Flags&ReadFlags == ReadFlags {
			p.pending--
			pd.Flags ^= ReadFlags
			pd.Cbs[ReadEvent](nil)
		}

		if flags&pd.Flags&WriteFlags == WriteFlags {
			p.pending--
			pd.Flags ^= WriteFlags
			pd.Cbs[WriteEvent](nil)
		}
	}

	return nil
}

func (p *Poller) SetRead(fd int, pd *PollData) error {
	return p.setRead(fd, syscall.EV_ADD|syscall.EV_ONESHOT, pd)
}

func (p *Poller) setRead(fd int, flags uint16, pd *PollData) error {
	pdflags := &pd.Flags
	if *pdflags&ReadFlags != ReadFlags {
		p.pending++
		*pdflags |= ReadFlags
		return p.set(fd, createEvent(flags, -ReadFlags, pd, 0))
	}
	return nil
}

func (p *Poller) SetWrite(fd int, pd *PollData) error {
	pdflags := &pd.Flags
	if *pdflags&WriteFlags != WriteFlags {
		p.pending++
		*pdflags |= WriteFlags
		return p.set(fd, createEvent(syscall.EV_ADD|syscall.EV_ONESHOT, -WriteFlags, pd, 0))
	}
	return nil
}

func (p *Poller) DelRead(fd int, pd *PollData) error {
	pdflags := &pd.Flags
	if *pdflags&ReadFlags == ReadFlags {
		p.pending--
		*pdflags ^= ReadFlags
		return p.set(fd, createEvent(syscall.EV_DELETE, -ReadFlags, pd, 0))
	}
	return nil
}

func (p *Poller) DelWrite(fd int, pd *PollData) error {
	pdflags := &pd.Flags
	if *pdflags&WriteFlags == WriteFlags {
		p.pending--
		*pdflags ^= WriteFlags
		return p.set(fd, createEvent(syscall.EV_DELETE, -WriteFlags, pd, 0))
	}
	return nil
}

func (p *Poller) Del(fd int, pd *PollData) error {
	err := p.DelRead(fd, pd)
	if err == nil {
		return p.DelWrite(fd, pd)
	}
	return nil
}

func (p *Poller) set(fd int, ev syscall.Kevent_t) error {
	ev.Ident = uint64(fd)
	p.changelist = append(p.changelist, ev)
	return nil
}

func createEvent(flags uint16, filter PollFlags, pd *PollData, dur time.Duration) syscall.Kevent_t {
	ev := syscall.Kevent_t{
		Flags:  flags,
		Filter: int16(filter),
	}

	if dur != 0 && (filter&syscall.EVFILT_TIMER == syscall.EVFILT_TIMER) {
		ev.Fflags = syscall.NOTE_NSECONDS
		ev.Data = dur.Nanoseconds()
	}

	if pd != nil {
		ev.Udata = (*byte)(unsafe.Pointer(pd)) // this is not touched by the kernel
	}

	return ev
}
