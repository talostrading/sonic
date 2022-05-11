package sonic

import (
	"errors"
	"net"
	"time"
)

type AsyncCallback func(error, int)

type File interface {
	Read([]byte) (int, error)
	Write([]byte) (int, error)

	// AsyncRead tries to read as much as possible into the provided buffer
	AsyncRead([]byte, AsyncCallback)

	// AsyncWrite tries to write as much as possible from the provided buffer
	AsyncWrite([]byte, AsyncCallback)

	// AsyncReadAll completes when the provided buffer is full
	AsyncReadAll([]byte, AsyncCallback)

	// AsyncWriteAll completes when the provided buffer is full
	AsyncWriteAll([]byte, AsyncCallback)

	Seek(offset int64, whence SeekWhence) error

	// Cancel cancells all async operations on the file
	Cancel()

	Close() error
}

// Conn is a stateful connection
// Conn tries to match the semantics of net.Conn as much as possible
type Conn interface {
	File

	LocalAddr() net.Addr
	RemoteAddr() net.Addr

	SetDeadline(t time.Time) error
	SetReadDeadline(t time.Time) error
	SetWriteDeadline(t time.Time) error
}

const (
	MaxReadDispatch  int = 512
	MaxWriteDispatch int = 512
)

var (
	ErrWouldBlock = errors.New("operation would block")
	ErrCancelled  = errors.New("operation cancelled")
	ErrTimeout    = errors.New("operation timed out")
)

type SeekWhence int

const (
	SeekStart SeekWhence = iota
	SeekCurrent
	SeekEnd
)
