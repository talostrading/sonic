package sonicerrors

import "errors"

var (
	ErrWouldBlock             = errors.New("operation would block")
	ErrCancelled              = errors.New("operation cancelled")
	ErrTimeout                = errors.New("operation timed out")
	ErrNeedMore               = errors.New("need to read/write more bytes")
	ErrNoBufferSpaceAvailable = errors.New("no buffer space available")
	ErrConnRefused            = errors.New("connection refused") // a connect() on a stream socket found no one listening on the remote address
)
