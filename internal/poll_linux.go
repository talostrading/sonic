//go:build linux

package internal

import (
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
	"sync/atomic"
	"syscall"
	"unsafe"

	"github.com/csdenboer/sonic/sonicerrors"
)

type PollerEvent uint32

const (
	PollerReadEvent  = PollerEvent(syscall.EPOLLIN)
	PollerWriteEvent = PollerEvent(syscall.EPOLLOUT)
)

func init() {
	// The read and write events are used to set/unset bits in a Slot's event mask. We dispatch the read/write handler
	// based on this event mask, so we must ensure they don't overlap.
	if PollerReadEvent|PollerWriteEvent == PollerReadEvent || PollerReadEvent|PollerWriteEvent == PollerWriteEvent {
		panic(fmt.Sprintf(
			"PollerReadEvent=%d and PollerWriteEvent=%d overlap",
			PollerReadEvent, PollerWriteEvent,
		))
	}
}

type Event struct {
	Mask uint32
	Data [8]byte
}

func createEvent(event PollerEvent, slot *Slot) Event {
	ev := Event{Mask: uint32(event)}
	/* #nosec G103 -- the use of unsafe has been audited */
	*(**Slot)(unsafe.Pointer(&ev.Data)) = slot
	return ev
}

var _ Poller = &poller{}

type poller struct {
	// fd is the file descriptor returned by calling epoll_create1(0).
	fd int

	// events contains the events which occurred.
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

	err = p.SetRead(p.waker.Slot())
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
	nn, _, errno := syscall.RawSyscall6(
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

		events := PollerEvent(event.Mask)
		/* #nosec G103 -- the use of unsafe has been audited */
		slot := *(**Slot)(unsafe.Pointer(&event.Data))

		if slot.Fd == p.waker.Fd() {
			p.dispatch()
			continue
		}

		if events&slot.Events&PollerReadEvent == PollerReadEvent {
			// TODO this errors should be reported
			_ = p.DelRead(slot)
			slot.Handlers[ReadEvent](nil)
		}

		if events&slot.Events&PollerWriteEvent == PollerWriteEvent {
			// TODO this errors should be reported
			_ = p.DelWrite(slot)
			slot.Handlers[WriteEvent](nil)
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

func (p *poller) SetRead(slot *Slot) error {
	return p.setRW(slot.Fd, slot, PollerReadEvent)
}

func (p *poller) SetWrite(slot *Slot) error {
	return p.setRW(slot.Fd, slot, PollerWriteEvent)
}

func (p *poller) setRW(fd int, slot *Slot, flag PollerEvent) error {
	events := &slot.Events
	if *events&flag != flag {
		p.pending++

		oldEvents := *events
		*events |= flag

		if oldEvents == 0 {
			return p.add(fd, createEvent(*events, slot))
		}
		return p.modify(fd, createEvent(*events, slot))
	}
	return nil
}

func (p *poller) add(fd int, event Event) error {
	/* #nosec G103 -- the use of unsafe has been audited */
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

func (p *poller) modify(fd int, event Event) error {
	/* #nosec G103 -- the use of unsafe has been audited */
	_, _, errno := syscall.RawSyscall6(
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

func (p *poller) Del(slot *Slot) error {
	err := p.DelRead(slot)
	if err == nil {
		return p.DelWrite(slot)
	}
	return nil
}

func (p *poller) DelRead(slot *Slot) error {
	events := &slot.Events
	if *events&PollerReadEvent == PollerReadEvent {
		p.pending--
		*events ^= PollerReadEvent
		if *events != 0 {
			return p.modify(slot.Fd, createEvent(*events, slot))
		}
		return p.del(slot.Fd)
	}
	return nil
}

func (p *poller) DelWrite(slot *Slot) error {
	events := &slot.Events
	if *events&PollerWriteEvent == PollerWriteEvent {
		p.pending--
		*events ^= PollerWriteEvent
		if *events != 0 {
			return p.modify(slot.Fd, createEvent(*events, slot))
		}
		return p.del(slot.Fd)
	}
	return nil
}

func (p *poller) del(fd int) error {
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
