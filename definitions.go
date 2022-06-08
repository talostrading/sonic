package sonic

import (
	"errors"
	"io"
	"net"
)

type AsyncCallback func(error, int)
type AcceptCallback func(error, Conn)

// AsyncReader is the interface that wraps the AsyncRead and AsyncReadAll methods.
type AsyncReader interface {
	// AsyncRead reads up to len(b) bytes into b asynchronously.
	//
	// This call should not block. The provided completion handler is called
	// in the following cases:
	//  - a read of n bytes completes
	//  - an error occurs
	//
	// Callers should always process the n > 0 bytes returned before considering
	// the error err.
	//
	// Implementation of AsyncRead are discouraged from invoking the handler
	// with a zero byte count with a nil error, except when len(b) == 0.
	// Callers should treat a return of 0 and nil as indicating that nothing
	// happened; in particular, it does not indicate EOF.
	//
	// Implementations must not retain b. Ownership of b must be retained by the caller,
	// which must guarantee that it remains valid until the handler is called.
	AsyncRead(b []byte, cb AsyncCallback)

	// AsyncReadAll reads len(b) bytes into b asynchronously.
	AsyncReadAll(b []byte, cb AsyncCallback)
}

// AsyncReader is the interface that wraps the AsyncRead and AsyncReadAll methods.
type AsyncWriter interface {
	// AsyncWrite writes up to len(b) bytes into the underlying data stream asynchronously.
	//
	// This call should not block. The provided completion handler is called
	// in the following cases:
	//  - a write of n bytes completes
	//  - an error occurs
	//
	// AsyncWrite must provide a non-nil error if it writes n < len(b) bytes.
	//
	// Implementations must not retain b. Ownership of b must be retained by the caller,
	// which must guarantee that it remains valid until the handler is called.
	// AsyncWrite must not modify b, even temporarily.
	AsyncWrite(b []byte, cb AsyncCallback)

	// AsyncReadAll writes len(b) bytes into the underlying data stream asynchronously.
	AsyncWriteAll(b []byte, cb AsyncCallback)
}

type AsyncReadWriter interface {
	AsyncReader
	AsyncWriter
}

type AsyncReaderFrom interface {
	AsyncReadFrom(AsyncReader, AsyncCallback)
}

type AsyncWriterTo interface {
	AsyncWriteTo(AsyncWriter, AsyncCallback)
}

// AsyncCloser is the interface that wraps the AsyncClose methods.
type AsyncCloser interface {
	// AsyncClose closes the underlying data stream asynchronously.
	//
	// This call does not block. If the close is successful, a nil
	// error should be provided.
	//
	// The behaviour of AsyncClose after the first call is undefined.
	// Specific implementations may document their own behaviour.
	AsyncClose(cb func(err error))
}

type FileDescriptor interface {
	io.Closer
	io.ReadWriter
	AsyncReadWriter

	// Closed returns true if the underlying file descriptor is closed.
	Closed() bool

	// Cancel cancells all asynchronous operations on the file descriptor.
	Cancel()
}

type File interface {
	FileDescriptor
	io.Seeker
}

// Stream represents a full-duplex connection between two programs or hosts,
// where data represented as bytes may be received reliably in the same order
// they were written.
type Stream interface {
	AsyncStream
	SyncStream
}

type AsyncStream interface {
	AsyncReadStream
	AsyncWriteStream
}

type AsyncReadStream interface {
	AsyncReader
	// TODO sergiu: maybe also: Executor() sonic.IO
	// also need to abstract sonic.IO if you want to support strands etc.
}

type AsyncWriteStream interface {
	AsyncWriter
	// TODO sergiu: maybe also: Executor() sonic.IO
}

type SyncStream interface {
	SyncReadStream
	SyncWriteStream
}

type SyncReadStream interface {
	io.Reader
	// TODO sergiu: maybe also: Executor() sonic.IO
}

type SyncWriteStream interface {
	io.Writer
	// TODO sergiu: maybe also: Executor() sonic.IO
}

// Conn is a generic stream-oriented network connection.
type Conn interface {
	FileDescriptor
	net.Conn
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
)
