package internal

import "errors"

var ErrTimeout = errors.New("operation timed out")

type EventType int8

const (
	ReadEvent EventType = iota
	WriteEvent
	MaxEvent
)

type Handler func(error)

type PollData struct {
	// Fd is the file descriptor associated with an instance of PollData
	// Fd is the unique identifier of PollData
	Fd    int
	Flags PollFlags
	Cbs   [MaxEvent]Handler
}

func (pd *PollData) Set(et EventType, h Handler) {
	pd.Cbs[et] = h
}
