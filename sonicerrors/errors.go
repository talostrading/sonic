package sonicerrors

import (
	"errors"
	"fmt"
)

var (
	ErrWouldBlock   = errors.New("operation would block")
	ErrCancelled    = errors.New("operation cancelled")
	ErrTimeout      = errors.New("operation timed out")
	ErrNeedMore     = errors.New("need more bytes")
	ErrReconnecting = errors.New("reconnecting")
)

type ErrReconnectingFailed struct {
	Actual    error
	Reconnect error
}

func (e *ErrReconnectingFailed) Error() string {
	return fmt.Sprintf("reconnecting failed actual_err=%v reconnect_err=%v", e.Actual, e.Reconnect)
}
