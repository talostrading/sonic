package internal

import "errors"

var (
	ErrWouldBlock = errors.New("operation would block")
	ErrCancelled  = errors.New("operation cancelled")
	ErrTimeout    = errors.New("operation timed out")
)
