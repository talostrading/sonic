//go:build linux

package internal

import (
	"io"
	"os"
	"sync"
	"sync/atomic"
	"syscall"
	"unsafe"
)

type PollFlags uint32

const (
	ReadFlags  = PollFlags(syscall.EPOLLIN)
	WriteFlags = PollFlags(syscall.EPOLLOUT)
)

type Event struct {
	Flags uint32
	Data  [8]byte
}

func createEvent(flags PollFlags, pd *PollData) Event {
	ev := Event{
		Flags: uint32(flags),
	}
	*(**PollData)(unsafe.Pointer(&ev.Data)) = pd

	return ev
}

type Poller struct {
	// fd is the file descriptor returned by calling epoll_create1(0).
	fd int

	// events contains the events which occured.
	// events is a subset of changelist.
	events []Event

	// waker is used to wake up the process when the client
	// calls ioc.Post(...), thus dispatching the provided handler.
	// The read end of the pipe is registered for reads with kqueue.
	waker *EventFd

	// handlers maintains the handlers set by the client to be
	// executed in the Poller's goroutine. Adding a handler
	// entails writing a single byte to the write end of the wakeupPipe.
	handlers []func()

	// lck synchronizes access to the handlers slice.
	// This is needed because multiple goroutines can call ioc.Post(...)
	// on the same IO object.
	lck sync.Mutex

	// pending is the number of pending handlers the poller needs to execute
	pending int64

	// closed is true if the close() has been called on fd
	closed uint32

	// TODO proper waker interface
	wakerBytes [8]byte
}

func NewPoller() (*Poller, error) {
	epollFd, err := syscall.EpollCreate1(0)
	if err != nil {
		return nil, err
	}

	eventFd, err := NewEventFd(true)
	if err != nil {
		return nil, err
	}

	p := &Poller{
		fd:     epollFd,
		waker:  eventFd,
		events: make([]Event, 128),
	}

	err = p.SetRead(p.waker.Fd(), p.waker.PollData())
	if err != nil {
		p.waker.Close()
		syscall.Close(p.fd)
		return nil, err
	}
	// ignore the waker
	p.pending--

	return p, err
}

func (p *Poller) Pending() int64 {
	return p.pending
}

func (p *Poller) Close() error {
	if !atomic.CompareAndSwapUint32(&p.closed, 0, 1) {
		return io.EOF
	}

	p.events = nil
	p.pending = 0

	p.waker.Close()
	return syscall.Close(p.fd)
}

func (p *Poller) Closed() bool {
	return atomic.LoadUint32(&p.closed) == 1
}

func (p *Poller) Post(handler func()) error {
	p.lck.Lock()
	p.handlers = append(p.handlers, handler)
	p.pending++
	p.lck.Unlock()

	_, err := p.waker.Write(1)
	return err
}

func (p *Poller) Poll(timeoutMs int) error {
	n, _, errno := syscall.RawSyscall6(
		syscall.SYS_EPOLL_WAIT,
		uintptr(p.fd),
		uintptr(unsafe.Pointer(&p.events[0])),
		uintptr(len(p.events)),
		uintptr(timeoutMs),
		0, 0,
	)
	if errno != 0 {
		return errno
	}

	if n == 0 && timeoutMs >= 0 {
		return ErrTimeout
	}

	for i := 0; i < int(n); i++ {
		event := &p.events[i]

		flags := PollFlags(event.Flags)
		pd := *(**PollData)(unsafe.Pointer(&event.Data))

		if pd.Fd == p.waker.Fd() {
			p.dispatch()
			continue
		}

		if flags&pd.Flags&ReadFlags == ReadFlags {
			p.DelRead(pd.Fd, pd)
			pd.Cbs[ReadEvent](nil)
		}

		if flags&pd.Flags&WriteFlags == WriteFlags {
			p.DelWrite(pd.Fd, pd)
			pd.Cbs[WriteEvent](nil)
		}
	}

	return nil
}

func (p *Poller) dispatch() {
	for {
		_, err := p.waker.Read(p.wakerBytes[:])
		if err != nil {
			break
		}
	}

	p.lck.Lock()
	for _, handler := range p.handlers {
		handler()
		p.pending--
	}
	p.lck.Unlock()
}

func (p *Poller) SetRead(fd int, pd *PollData) error {
	return p.setRW(fd, pd, ReadFlags)
}

func (p *Poller) SetWrite(fd int, pd *PollData) error {
	return p.setRW(fd, pd, WriteFlags)
}

func (p *Poller) setRW(fd int, pd *PollData, flag PollFlags) error {
	pdflags := &pd.Flags
	if *pdflags&flag != flag {
		p.pending++

		oldFlags := *pdflags
		*pdflags |= flag

		if oldFlags == 0 {
			return p.add(fd, createEvent(*pdflags, pd))
		}
		return p.modify(fd, createEvent(*pdflags, pd))
	}
	return nil
}

func (p *Poller) add(fd int, event Event) error {
	_, _, errno := syscall.RawSyscall6(
		syscall.SYS_EPOLL_CTL,
		uintptr(p.fd),
		uintptr(syscall.EPOLL_CTL_ADD),
		uintptr(fd),
		uintptr(unsafe.Pointer(&event)),
		0, 0,
	)
	if errno != 0 {
		return os.NewSyscallError("epoll_ctl_add", errno)
	}
	return nil
}

func (p *Poller) modify(fd int, event Event) error {
	_, _, errno := syscall.RawSyscall6(
		syscall.SYS_EPOLL_CTL,
		uintptr(p.fd),
		uintptr(syscall.EPOLL_CTL_MOD),
		uintptr(fd),
		uintptr(unsafe.Pointer(&event)),
		0, 0,
	)

	if errno != 0 {
		return os.NewSyscallError("epoll_ctl_add", errno)
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

func (p *Poller) DelRead(fd int, pd *PollData) error {
	pdflags := &pd.Flags
	if *pdflags&ReadFlags == ReadFlags {
		p.pending--
		*pdflags ^= ReadFlags
		if *pdflags != 0 {
			return p.modify(fd, createEvent(*pdflags, pd))
		}
		return p.del(fd)
	}
	return nil
}

func (p *Poller) DelWrite(fd int, pd *PollData) error {
	pdflags := &pd.Flags
	if *pdflags&WriteFlags == WriteFlags {
		p.pending--
		*pdflags ^= WriteFlags
		if *pdflags != 0 {
			return p.modify(fd, createEvent(*pdflags, pd))
		}
		return p.del(fd)
	}
	return nil
}

func (p *Poller) del(fd int) error {
	_, _, errno := syscall.RawSyscall6(
		syscall.SYS_EPOLL_CTL,
		uintptr(p.fd),
		uintptr(syscall.EPOLL_CTL_DEL),
		uintptr(fd),
		0, 0, 0,
	)
	if errno != 0 {
		return os.NewSyscallError("epoll_ctl_del", errno)
	}
	return nil
}
