package sonic

import (
	"io"
	"net"
)

type AsyncCallback func(error, int)
type AcceptCallback func(error, Conn)
type AcceptPacketCallback func(error, PacketConn)

// AsyncReader is the interface that wraps the AsyncRead and AsyncReadAll methods.
type AsyncReader interface {
	// AsyncRead reads up to `len(b)` bytes into `b` asynchronously.
	//
	// This call should not block. The provided completion handler is called in the following cases:
	// - a read of up to `n` bytes completes
	// - an error occurs
	//
	// Callers should always process the returned bytes before considering the error err. Implementations of AsyncRead
	// are discouraged from invoking the handler with 0 bytes and a nil error.
	//
	// Ownership of the byte slice must be retained by callers, which must guarantee that it remains valid until the
	// callback is invoked.
	AsyncRead(b []byte, cb AsyncCallback)

	// AsyncReadAll reads exactly `len(b)` bytes into b asynchronously.
	AsyncReadAll(b []byte, cb AsyncCallback)
}

// AsyncWriter is the interface that wraps the AsyncWrite and AsyncWriteAll methods.
type AsyncWriter interface {
	// AsyncWrite writes up to `len(b)` bytes from `b` asynchronously.
	//
	// This call does not block. The provided completion handler is called in the following cases:
	//  - a write of up to `n` bytes completes
	//  - an error occurs
	//
	// Ownership of the byte slice must be retained by callers, which must guarantee that it remains valid until the
	// callback is invoked. This function does not modify the given byte slice, even temporarily.
	AsyncWrite(b []byte, cb AsyncCallback)

	// AsyncWriteAll writes exactly `len(b)` bytes into the underlying data stream asynchronously.
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

type FileDescriptor interface {
	RawFd() int

	io.Closer
	io.ReadWriter
	AsyncReadWriter
	AsyncCanceller
}

type File interface {
	FileDescriptor
	io.Seeker
}

type AsyncCanceller interface {
	// Cancel cancells all asynchronous operations on the next layer.
	Cancel()
}

// Stream represents a full-duplex connection between two processes, where data represented as bytes may be received
// reliably in the same order they were written.
type Stream interface {
	RawFd() int

	AsyncStream
	SyncStream
}

type AsyncStream interface {
	AsyncReadStream
	AsyncWriteStream
	io.Closer
}

type AsyncReadStream interface {
	AsyncReader
	AsyncCanceller
	io.Closer
}

type AsyncWriteStream interface {
	AsyncWriter
	AsyncCanceller
	io.Closer
}

type SyncStream interface {
	SyncReadStream
	SyncWriteStream
	io.Closer
}

type SyncReadStream interface {
	io.Reader
	io.Closer
}

type SyncWriteStream interface {
	io.Writer
	io.Closer
}

// Conn is a generic stream-oriented network connection.
type Conn interface {
	FileDescriptor
	net.Conn
}

type AsyncReadCallbackPacket func(error, int, net.Addr)
type AsyncWriteCallbackPacket func(error)

// PacketConn is a generic packet-oriented connection.
type PacketConn interface {
	ReadFrom([]byte) (n int, addr net.Addr, err error)
	AsyncReadFrom([]byte, AsyncReadCallbackPacket)
	AsyncReadAllFrom([]byte, AsyncReadCallbackPacket)

	WriteTo([]byte, net.Addr) error
	AsyncWriteTo([]byte, net.Addr, AsyncWriteCallbackPacket)

	Close() error
	Closed() bool

	LocalAddr() net.Addr
	RawFd() int
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
	Addr() net.Addr

	RawFd() int
}
