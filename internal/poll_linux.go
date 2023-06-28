//go:build linux

package internal

import (
	"errors"
	"io"
	"os"
	"sync"
	"sync/atomic"
	"syscall"
	"unsafe"

	"github.com/talostrading/sonic/sonicerrors"
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
	/* #nosec G103 -- the use of unsafe has been audited */
	*(**PollData)(unsafe.Pointer(&ev.Data)) = pd

	return ev
}

var _ Poller = &poller{}

type poller struct {
	// fd is the file descriptor returned by calling epoll_create1(0).
	fd int

	// events contains the events which occured.
	// events is a subset of changelist.
	events []Event

	// waker is used to wake up the process when the client
	// calls ioc.Post(...), thus dispatching the provided handler.
	// The read end of the pipe is registered for reads with kqueue.
	waker *EventFd

	// posts maintains the posts set by the client to be
	// executed in the poller's goroutine. Adding a handler
	// entails writing a single byte to the write end of the wakeupPipe.
	posts []func()

	// lck synchronizes access to the posts slice.
	// This is needed because multiple goroutines can call ioc.Post(...)
	// on the same IO object.
	lck sync.Mutex

	// pending is the number of pending posts the poller needs to execute
	pending int64

	// closed is true if the close() has been called on fd
	closed uint32

	// TODO proper waker interface
	wakerBytes [8]byte
}

func NewPoller() (Poller, error) {
	epollFd, err := syscall.EpollCreate1(0)
	if err != nil {
		return nil, err
	}

	eventFd, err := NewEventFd(true)
	if err != nil {
		_ = syscall.Close(epollFd)
		return nil, err
	}

	p := &poller{
		fd:     epollFd,
		waker:  eventFd,
		events: make([]Event, 128),
	}

	err = p.SetRead(p.waker.Fd(), p.waker.PollData())
	if err != nil {
		_ = p.waker.Close()
		_ = syscall.Close(p.fd)
		return nil, err
	}
	// ignore the waker
	p.pending--

	return p, err
}

func (p *poller) Pending() int64 {
	return p.pending
}

func (p *poller) Close() error {
	if !atomic.CompareAndSwapUint32(&p.closed, 0, 1) {
		return io.EOF
	}

	_ = p.waker.Close()
	return syscall.Close(p.fd)
}

func (p *poller) Closed() bool {
	return atomic.LoadUint32(&p.closed) == 1
}

func (p *poller) Post(handler func()) error {
	p.lck.Lock()
	p.posts = append(p.posts, handler)
	p.pending++
	p.lck.Unlock()

	// Concurrent writes are thread safe for eventfds.
	_, err := p.waker.Write(1)
	return err
}

func (p *poller) Posted() int {
	p.lck.Lock()
	defer p.lck.Unlock()

	return len(p.posts)
}

func (p *poller) Poll(timeoutMs int) (n int, err error) {
	/* #nosec G103 -- the use of unsafe has been audited */
	nn, _, errno := syscall.Syscall6(
		syscall.SYS_EPOLL_WAIT,
		uintptr(p.fd),
		uintptr(unsafe.Pointer(&p.events[0])),
		uintptr(len(p.events)),
		uintptr(timeoutMs),
		0, 0,
	)
	n = int(nn)

	if errno != 0 {
		err = errno // we need to convert
	}

	if err != nil {
		return n, err
	}

	// n == -1 and errno == 0 might happen if we epoll_wait on a closed epoll fd
	if n < 0 {
		return n, errors.New("unknown epoll_wait error")
	}

	if n == 0 && timeoutMs >= 0 {
		return n, sonicerrors.ErrTimeout
	}

	for i := 0; i < int(n); i++ {
		event := &p.events[i]

		flags := PollFlags(event.Flags)
		/* #nosec G103 -- the use of unsafe has been audited */
		pd := *(**PollData)(unsafe.Pointer(&event.Data))

		if pd.Fd == p.waker.Fd() {
			p.dispatch()
			continue
		}

		if flags&pd.Flags&ReadFlags == ReadFlags {
			// TODO this errors should be reported
			_ = p.DelRead(pd.Fd, pd)
			pd.Cbs[ReadEvent](nil)
		}

		if flags&pd.Flags&WriteFlags == WriteFlags {
			// TODO this errors should be reported
			_ = p.DelWrite(pd.Fd, pd)
			pd.Cbs[WriteEvent](nil)
		}
	}

	return n, nil
}

func (p *poller) dispatch() {
	for {
		_, err := p.waker.Read(p.wakerBytes[:])
		if err != nil {
			break
		}
	}

	p.lck.Lock()
	for _, handler := range p.posts {
		handler()
		p.pending--
	}
	p.posts = p.posts[:0]
	p.lck.Unlock()
}

func (p *poller) SetRead(fd int, pd *PollData) error {
	return p.setRW(fd, pd, ReadFlags)
}

func (p *poller) SetWrite(fd int, pd *PollData) error {
	return p.setRW(fd, pd, WriteFlags)
}

func (p *poller) setRW(fd int, pd *PollData, flag PollFlags) error {
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

func (p *poller) add(fd int, event Event) error {
	/* #nosec G103 -- the use of unsafe has been audited */
	_, _, errno := syscall.Syscall6(
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

func (p *poller) modify(fd int, event Event) error {
	/* #nosec G103 -- the use of unsafe has been audited */
	_, _, errno := syscall.Syscall6(
		syscall.SYS_EPOLL_CTL,
		uintptr(p.fd),
		uintptr(syscall.EPOLL_CTL_MOD),
		uintptr(fd),
		uintptr(unsafe.Pointer(&event)),
		0, 0,
	)

	if errno != 0 {
		return os.NewSyscallError("epoll_ctl_mod", errno)
	}
	return nil
}

func (p *poller) Del(fd int, pd *PollData) error {
	err := p.DelRead(fd, pd)
	if err == nil {
		return p.DelWrite(fd, pd)
	}
	return nil
}

func (p *poller) DelRead(fd int, pd *PollData) error {
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

func (p *poller) DelWrite(fd int, pd *PollData) error {
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

func (p *poller) del(fd int) error {
	_, _, errno := syscall.Syscall6(
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
