//go:build darwin || netbsd || freebsd || openbsd || dragonfly

package internal

import (
	"errors"
	"fmt"
	"io"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"

	"github.com/talostrading/sonic/sonicerrors"
)

var oneByte = [1]byte{0}

type PollerEvent int16

const (
	PollerReadEvent  = -PollerEvent(syscall.EVFILT_READ)
	PollerWriteEvent = -PollerEvent(syscall.EVFILT_WRITE)
)

func init() {
	if PollerReadEvent == PollerWriteEvent {
		panic("poller read event == poller write event")
	}

	// The read and write events are used to set/unset bits in a Slot's event mask. We dispatch the read/write handler
	// based on this event mask, so we must ensure these are powers of two.
	if !IsPowerOfTwo(int(PollerReadEvent)) {
		panic(fmt.Sprintf("PollerReadEvent=%d is not a power of two", PollerReadEvent))
	}
	if !IsPowerOfTwo(int(PollerWriteEvent)) {
		panic(fmt.Sprintf("PollerWriteEvent=%d is not a power of two", PollerWriteEvent))
	}
}

var _ Poller = &poller{}

type poller struct {
	// fd is the file descriptor returned by calling kqueue().
	fd int

	// changes contains events we want to watch for
	changes []syscall.Kevent_t

	// events contains the events which occurred.
	// events is a subset of changelist.
	events []syscall.Kevent_t

	// waker is used to wake up the process when the client
	// calls ioc.Post(...), thus dispatching the provided handler.
	// The read end of the pipe is registered for reads with kqueue.
	waker *Pipe

	// posts maintains the posts set by the client to be
	// executed in the poller's goroutine. Adding a handler
	// entails writing a single byte to the write end of the wakeupPipe.
	posts []func()

	// lck synchronizes access to the handlers slice.
	// This is needed because multiple goroutines can call ioc.Post(...)
	// on the same IO object.
	lck sync.Mutex

	// pending is the number of pending handlers the poller needs to execute
	pending int64

	// closed is true if the close() has been called on fd
	closed uint32
}

func NewPoller() (Poller, error) {
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

	kqueueFd, err := syscall.Kqueue()
	if err != nil {
		return nil, err
	}

	_, err = syscall.Kevent(kqueueFd, []syscall.Kevent_t{{
		Ident:  uint64(kqueueFd),
		Filter: syscall.EVFILT_USER,
		Flags:  syscall.EV_ADD | syscall.EV_CLEAR,
	}}, nil, nil)
	if err != nil {
		_ = pipe.Close()
		_ = syscall.Close(kqueueFd)
		return nil, err
	}

	p := &poller{
		waker:   pipe,
		fd:      kqueueFd,
		changes: make([]syscall.Kevent_t, 0, 128),
		events:  make([]syscall.Kevent_t, 128),
	}

	err = p.setRead(p.waker.ReadFd(), syscall.EV_ADD, &p.waker.slot)
	if err != nil {
		_ = p.waker.Close()
		_ = syscall.Close(kqueueFd)
		return nil, err
	}
	p.pending-- // ignore the pipe read

	return p, nil
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

	// Concurrent writes are thread safe for pipes if less
	// than 512 bytes are written.
	_, err := p.waker.Write(oneByte[:])
	return err
}

func (p *poller) Posted() int {
	p.lck.Lock()
	defer p.lck.Unlock()

	return len(p.posts)
}

func (p *poller) Poll(timeoutMs int) (n int, err error) {
	var timeout *syscall.Timespec
	if timeoutMs >= 0 {
		ts := syscall.NsecToTimespec(int64(timeoutMs) * 1e6)
		timeout = &ts
	}

	changelist := p.changes
	p.changes = p.changes[:0]

	n, err = syscall.Kevent(p.fd, changelist, p.events, timeout)

	if err != nil {
		return n, err
	}

	// This should never happen on BSD, but it does on Linux, so we check it
	// here aswell.
	if n < 0 {
		return n, errors.New("unknown kevent error")
	}

	if n == 0 && timeoutMs >= 0 {
		return n, sonicerrors.ErrTimeout
	}

	for i := 0; i < n; i++ {
		event := &p.events[i]

		events := -PollerEvent(event.Filter)

		/* #nosec G103 -- the use of unsafe has been audited */
		slot := (*Slot)(unsafe.Pointer(event.Udata))

		if slot.Fd == p.waker.ReadFd() {
			p.executePost()
			continue
		}

		if events&slot.Events&PollerReadEvent == PollerReadEvent {
			if !slot.Handlers[ReadEvent].multishot {
				p.pending--
				slot.Events ^= PollerReadEvent
			}
			slot.Handlers[ReadEvent].fn(nil)
		}

		if events&slot.Events&PollerWriteEvent == PollerWriteEvent {
			if !slot.Handlers[WriteEvent].multishot {
				p.pending--
				slot.Events ^= PollerWriteEvent
			}
			slot.Handlers[WriteEvent].fn(nil)
		}
	}

	return n, nil
}

func (p *poller) executePost() {
	for {
		_, err := p.waker.Read(oneByte[:])
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
	return p.setRead(slot.Fd, syscall.EV_ADD|syscall.EV_ONESHOT, slot)
}

func (p *poller) SetReadMulti(slot *Slot) error {
	return p.setRead(slot.Fd, syscall.EV_ADD, slot)
}

func (p *poller) setRead(fd int, flags uint16, slot *Slot) error {
	events := &slot.Events
	if *events&PollerReadEvent != PollerReadEvent {
		p.pending++
		*events |= PollerReadEvent
		return p.set(fd, createEvent(flags, -PollerReadEvent, slot, 0))
	}
	return nil
}

func (p *poller) SetWrite(slot *Slot) error {
	return p.setWrite(slot.Fd, syscall.EV_ADD|syscall.EV_ONESHOT, slot)
}

func (p *poller) SetWriteMulti(slot *Slot) error {
	return p.setWrite(slot.Fd, syscall.EV_ADD, slot)
}

func (p *poller) setWrite(fd int, flags uint16, slot *Slot) error {
	events := &slot.Events
	if *events&PollerWriteEvent != PollerWriteEvent {
		p.pending++
		*events |= PollerWriteEvent
		return p.set(fd, createEvent(flags, -PollerWriteEvent, slot, 0))
	}
	return nil
}

func (p *poller) DelRead(slot *Slot) error {
	events := &slot.Events
	if *events&PollerReadEvent == PollerReadEvent {
		p.pending--
		*events ^= PollerReadEvent
		return p.set(slot.Fd, createEvent(syscall.EV_DELETE, -PollerReadEvent, slot, 0))
	}
	return nil
}

func (p *poller) DelWrite(slot *Slot) error {
	events := &slot.Events
	if *events&PollerWriteEvent == PollerWriteEvent {
		p.pending--
		*events ^= PollerWriteEvent
		return p.set(slot.Fd, createEvent(syscall.EV_DELETE, -PollerWriteEvent, slot, 0))
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

func (p *poller) set(fd int, ev syscall.Kevent_t) error {
	ev.Ident = uint64(fd)
	p.changes = append(p.changes, ev)
	return nil
}

func createEvent(flags uint16, filter PollerEvent, slot *Slot, dur time.Duration) syscall.Kevent_t {
	ev := syscall.Kevent_t{
		Flags:  flags,
		Filter: int16(filter),
	}

	if dur != 0 && (filter&syscall.EVFILT_TIMER == syscall.EVFILT_TIMER) {
		ev.Fflags = syscall.NOTE_NSECONDS
		ev.Data = dur.Nanoseconds()
	}

	if slot != nil {
		/* #nosec G103 -- the use of unsafe has been audited */
		ev.Udata = (*byte)(unsafe.Pointer(slot)) // not touched by the kernel
	}

	return ev
}
