package sonic

import (
	"io"
	"net"
)

type AsyncCallback func(error, int)

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

	// CancelReads cancels all pending asynchronous reads.
	CancelReads()
}

// AsyncWriter is the interface that wraps the AsyncRead and AsyncReadAll methods.
type AsyncWriter interface {
	// AsyncWrite writes up to len(b) bytes asynchronously.
	//
	// This call should not block. The provided completion handler is called in the following cases:
	//  - a write of n bytes completes
	//  - an error occurs
	//
	// AsyncWrite must provide a non-nil error if it writes n < len(b) bytes.
	//
	// Implementations must not retain b. Ownership of b must be retained by the caller,
	// which must guarantee that it remains valid until the handler is called.
	// AsyncWrite must not modify b, even temporarily.
	AsyncWrite(b []byte, cb AsyncCallback)

	// AsyncWriteAll writes len(b) bytes asynchronously.
	AsyncWriteAll(b []byte, cb AsyncCallback)

	// CancelWrites cancels all pending asynchronous writes.
	CancelWrites()
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

type FileDescriptor interface {
	io.Closer
	Closed() bool

	io.ReadWriter
	AsyncReadWriter

	RawFd() int
}

type File interface {
	FileDescriptor
	io.Seeker
}

// Conn is a generic stream-oriented network connection.
type Conn interface {
	FileDescriptor
	net.Conn
}

// PacketConn is a generic packet-oriented network connection.
type PacketConn interface {
	FileDescriptor
	net.PacketConn
}

type AcceptCallback func(error, Conn)

// Listener is a generic network listener for stream-oriented protocols.
type Listener interface {
	// Accept waits for and returns the next connection to the listener.
	Accept() (Conn, error)

	// AsyncAccept waits for and returns the next connection to the listener asynchronously.
	AsyncAccept(AcceptCallback)

	// Close closes the listener.
	//
	// Any blocked Accept operations will be unblocked and return errors.
	// Any pending AsyncAccept operations will be cancelled and report errors.
	Close() error

	// Addr returns the listener's network address.
	Addr() net.Addr
}

const (
	// MaxCallbackDispatch is the maximum number of callbacks which can be
	// placed onto the stack for immediate invocation.
	MaxCallbackDispatch int = 1024
)
