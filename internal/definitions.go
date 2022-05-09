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
	Flags PollFlags
	Cbs   [MaxEvent]Handler
}

func (pd *PollData) Set(et EventType, h Handler) {
	pd.Cbs[et] = h
}
