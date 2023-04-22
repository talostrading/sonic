package internal

import "time"

type EventType int8

const (
	ReadEvent EventType = iota
	WriteEvent
	MaxEvent
)

type Handler func(error)

type PollData struct {
	// Fd is the file descriptor associated with an instance of PollData
	Fd  int               // must set by callers
	Cbs [MaxEvent]Handler // must set by callers

	Flags PollFlags // must NOT be set by callers, instead set by the platform specific poller
}

func (pd *PollData) Set(et EventType, h Handler) {
	pd.Cbs[et] = h
}

type ITimer interface {
	Set(time.Duration, func()) error
	Unset() error
	Close() error
}

type Poller interface {
	// Poll polls the status of the underlying events registered with
	// SetRead or SetWrite, checking if any occured.
	//
	// A call to Poll will block until either:
	//  - an event occurs
	//  - the call is interrupted by a signal handler; or
	//  - the timeout expires
	Poll(timeoutMs int) (n int, err error)

	// Pending returns the number of registered events which have not yet
	// occured.
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
	SetRead(fd int, pd *PollData) error

	// SetWrite registers interest in write events on the provided
	// file descriptor.
	SetWrite(fd int, pd *PollData) error

	// DelRead deregisters interest in read events on the provided
	// file descriptor.
	DelRead(fd int, pd *PollData) error

	// DelWrite deregisters interest in write events on the provided
	// file descriptor.
	DelWrite(fd int, pd *PollData) error

	// Del deregisters interest in all events on the provided file descriptor.
	Del(fd int, pd *PollData) error

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
