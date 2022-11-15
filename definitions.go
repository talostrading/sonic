package sonic

import (
	"github.com/talostrading/sonic/sonicopts"
	"io"
	"net"
)

const (
	// MaxCallbackDispatch is the maximum number of callbacks which can be
	// placed onto the stack for immediate invocation. Any additional callback
	// will be scheduled to run asynchronously.
	MaxCallbackDispatch int = 1024
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

// FileDescriptor is a POSIX file descriptor i.e. a unique identifier for a file or other input/output resource,
// such as a pipe or network socket.
type FileDescriptor interface {
	io.ReadWriter
	AsyncReadWriter

	io.Closer
	Closed() bool

	RawFd() int

	Opts() []sonicopts.Option
}

// File is POSIX file.
type File interface {
	FileDescriptor
	io.Seeker
}

type Socket interface {
	FileDescriptor

	Opts() []sonicopts.Option
	SetOpts(...sonicopts.Option)

	RecvMsg() ([]byte, error)

	RawFd() int // TODO move this to FileDescriptor

	RemoteAddr() net.Addr
	LocalAddr() net.Addr
}

// Conn is a generic stream-oriented network connection.
type Conn interface {
	FileDescriptor
	net.Conn

	// TODO Layered[Socket]
}

// PacketConn is a generic packet-oriented network connection.
type PacketConn interface {
	FileDescriptor
	net.PacketConn

	// TODO Layered[Socket]
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

	// TODO Layered[Socket]
}

type Encoder[Item any] interface {
	// Encode encodes the given `Item` into the `dst` buffer.
	Encode(item Item, dst *ByteBuffer) error
}

type Decoder[Item any] interface {
	// Decode decodes the read area of the buffer into an `Item`.
	//
	// An implementation of Codec takes a byte stream that has already
	// been buffered in `src` and decodes the data into `Item`s.
	//
	// Implementations should return an empty Item and ErrNeedMore if
	// there are not enough bytes to decode into an Item.
	Decode(src *ByteBuffer) (Item, error)
}

// Codec defines a generic interface through which one can encode/decode raw bytes.
//
// Implementations are optionally able to track their state which enables
// writing both stateful and stateless parsers.
type Codec[Enc, Dec any] interface {
	Encoder[Enc]
	Decoder[Dec]
}

// CodecConn defines a generic interface through which a stream
// of bytes from Conn can be encoded/decoded with a Codec.
type CodecConn[Enc, Dec any] interface {
	ReadNext() (Dec, error)
	AsyncReadNext(func(error, Dec))

	WriteNext(Enc) error
	AsyncWriteNext(Enc, func(error))

	Layered[Conn]

	io.Closer
	Closed() bool
}

// Layered represents any type of network or file entity that sits on top of another entity which we call a layer.
type Layered[T any] interface {
	NextLayer() T
}

// ReconnectingConn maintains a persistent connection to an endpoint, by always reconnecting to the peer in case of
// a disconnect.
//
// In case reads/writes are done while reconnecting, an ErrReconnecting is returned to the caller.
type ReconnectingConn interface {
	Conn
	Layered[Conn]

	Reconnect() error
	AsyncReconnect(func(error))

	SetOnReconnect(func())
}
