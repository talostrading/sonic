package sonic

import (
	"io"
	"net"
)

const (
	// MaxCallbackDispatch is the maximum number of callbacks which can be
	// placed onto the stack for immediate invocation.
	MaxCallbackDispatch int = 32
)

// TODO this is quite a mess right now. Properly define what a Conn, Stream,
// PacketConn is and what should the async adapter return, as more than TCP
// streams can be "async-adapted".

type AsyncCallback func(error, int)
type AcceptCallback func(error, Conn)
type AcceptPacketCallback func(error, PacketConn)

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

// AsyncWriter is the interface that wraps the AsyncRead and AsyncReadAll methods.
type AsyncWriter interface {
	// AsyncWrite writes up to len(b) bytes into the underlying data stream asynchronously.
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

	// AsyncWriteAll writes len(b) bytes into the underlying data stream asynchronously.
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

// Stream represents a full-duplex connection between two processes,
// where data represented as bytes may be received reliably in the same order
// they were written.
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

// UDPMulticastClient defines a UDP multicast client that can read data from one or multiple multicast groups,
// optionally filtering packets on the source IP.
type UDPMulticastClient interface {
	Join(multicastAddr *net.UDPAddr) error
	JoinSource(multicastAddr, sourceAddr *net.UDPAddr) error

	Leave(multicastAddr *net.UDPAddr) error
	LeaveSource(multicastAddr, sourceAddr *net.UDPAddr) error

	BlockSource(multicastAddr, sourceAddr *net.UDPAddr) error
	UnblockSource(multicastAddr, sourceAddr *net.UDPAddr) error

	// ReadFrom and AsyncReadFrom read a partial or complete datagram into the provided buffer. When the datagram
	// is smaller than the passed buffer, only that much data is returned; when it is bigger, the packet is truncated
	// to fit into the buffer. The truncated bytes are lost.
	//
	// It is the responsibility of the caller to ensure the passed buffer can hold an entire datagram. A rule of thumb
	// is to have the buffer size equal to the network interface's MTU.
	ReadFrom([]byte) (n int, addr net.Addr, err error)
	AsyncReadFrom([]byte, AsyncReadCallbackPacket)

	RawFd() int
	Interface() *net.Interface
	LocalAddr() *net.UDPAddr

	Close() error
	Closed() bool
}
