package sonic

import (
	"errors"
	"net"
	"time"
)

type AsyncCallback func(error, int)
type AcceptCallback func(error, Conn)

type AsyncReader interface {
	AsyncRead([]byte, AsyncCallback)
	AsyncReadAll([]byte, AsyncCallback)
}

type AsyncWriter interface {
	AsyncWrite([]byte, AsyncCallback)
	AsyncWriteAll([]byte, AsyncCallback)
}

type AsyncReadWriter interface {
	AsyncReader
	AsyncWriter
}

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
	Closed() bool
}

// Conn is a generic stream-oriented network connection.
type Conn interface {
	File

	LocalAddr() net.Addr
	RemoteAddr() net.Addr

	SetDeadline(t time.Time) error
	SetReadDeadline(t time.Time) error
	SetWriteDeadline(t time.Time) error
}

// TODO
type PacketConn interface {
}

// Listener is a generic network listener for stream-oriented protocols.
type Listener interface {
	// Accept waits for and returns the next connection to the listener synchronously.
	Accept() (Conn, error)

	// AsyncAccept waits for and returns the next connection to the listener asynchronously.
	AsyncAccept(AcceptCallback)

	// Close closes the listener.
	Close() error

	// Addr returns the listener's network address.
	Addr() error
}

const (
	MaxReadDispatch   int = 512
	MaxWriteDispatch  int = 512
	MaxAcceptDispatch int = 512
)

var (
	ErrWouldBlock = errors.New("operation would block")
	ErrCancelled  = errors.New("operation cancelled")
	ErrTimeout    = errors.New("operation timed out")
	ErrEOF        = errors.New("end of file")
)

type SeekWhence int

const (
	SeekStart SeekWhence = iota
	SeekCurrent
	SeekEnd
)
