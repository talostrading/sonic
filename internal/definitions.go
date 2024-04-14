package internal

import "time"

type EventType int8

const (
	ReadEvent EventType = iota
	WriteEvent
	MaxEvent
)

type Handler func(error)

type Slot struct {
	Fd int // A file descriptor which uniquely identifies a Slot. Callers must set it up at construction time.

	// Events registered with this Slot. Essentially a bitmask. It can contain a read event, a write event, or both.
	// Every event from here has a corresponding Handler in Handlers.
	//
	// Defined by Poller, which is platform-specific. Since this is a bitmask, the Poller guarantees that each
	// platform-specific event is a power of two.
	Events PollerEvent

	// Callbacks registered with this Slot. The poller dispatches the appropriate read or write callback when it
	// receives an event that's in Events.
	Handlers [MaxEvent]Handler
}

func (s *Slot) Set(et EventType, h Handler) {
	s.Handlers[et] = h
}

type ITimer interface {
	Set(time.Duration, func()) error
	Unset() error
	Close() error
}

type Poller interface {
	// Poll polls the status of the underlying events registered with
	// SetRead or SetWrite, checking if any occurred.
	//
	// A call to Poll will block until either:
	//  - an event occurs
	//  - the call is interrupted by a signal handler; or
	//  - the timeout expires
	Poll(timeoutMs int) (n int, err error)

	// Pending returns the number of registered events which have not yet
	// occurred.
	Pending() int64

	// Post instructs the Poller to execute the provided handler in the Poller's
	// goroutine in the next Poll call.
	//
	// Post is safe for concurrent use by multiple goroutines.
	Post(func()) error

	// Posted returns the number of handlers registered with Post.
	//
	// Posted is safe for concurrent use by multiple goroutines.
	Posted() int

	// SetRead registers interest in read events on the provided
	// file descriptor.
	SetRead(slot *Slot) error

	// SetWrite registers interest in write events on the provided
	// file descriptor.
	SetWrite(slot *Slot) error

	// DelRead deregisters interest in read events on the provided
	// file descriptor.
	DelRead(slot *Slot) error

	// DelWrite deregisters interest in write events on the provided
	// file descriptor.
	DelWrite(slot *Slot) error

	// Del deregisters interest in all events on the provided file descriptor.
	Del(slot *Slot) error

	// Close closes the Poller.
	//
	// No calls to Poll should be make after Close.
	//
	// Close is safe for concurrent use by multiple goroutines.
	Close() error

	// Closed returns true if the Poller has been closed.
	//
	// Closed is safe for concurrent use by multiple goroutines.
	Closed() bool
}
