package websocket

import (
	"time"

	"github.com/talostrading/sonic"
)

var (
	MaxMessageSize = 1024 * 512 // the maximum size of a message
	CloseTimeout   = 5 * time.Second
)

type Role uint8

const (
	RoleClient Role = iota
	RoleServer
)

func (r Role) String() string {
	switch r {
	case RoleClient:
		return "role_client"
	case RoleServer:
		return "role_client"
	default:
		return "role_unknown"
	}
}

type MessageType uint8

const (
	TypeText   = MessageType(OpcodeText)
	TypeBinary = MessageType(OpcodeBinary)
	TypeClose  = MessageType(OpcodeClose)
	TypePing   = MessageType(OpcodePing)
	TypePong   = MessageType(OpcodePong)

	TypeNone MessageType = 0xFF
)

func (t MessageType) String() string {
	switch t {
	case TypeText:
		return "type_text"
	case TypeBinary:
		return "type_binary"
	case TypeClose:
		return "type_close"
	case TypePing:
		return "type_ping"
	case TypePong:
		return "type_pong"
	default:
		return "type_unknown"
	}
}

type StreamState uint8

const (
	// start state - handshake is ongoing.
	StateHandshake StreamState = iota

	// intermediate state - connection is active
	StateActive

	// intermediate state - we initiated the close handshake and
	// are waiting for a reply from the peer.
	StateClosedByUs

	// terminal state - the peer initiated the close handshake,
	// we received a close frame and immediately replied.
	StateClosedByPeer

	// terminal state - the peer replied to our close handshake.
	// Can only end up here from StateClosedByUs.
	StateCloseAcked

	// terminal state - the connection is closed or some error
	// occurred which rendered the stream unusable.
	StateTerminated
)

func (s StreamState) String() string {
	switch s {
	case StateHandshake:
		return "state_handshake"
	case StateActive:
		return "state_active"
	case StateClosedByUs:
		return "state_closed_by_us"
	case StateClosedByPeer:
		return "state_closed_by_peer"
	case StateCloseAcked:
		return "state_closed_acked"
	case StateTerminated:
		return "state_terminated"
	default:
		return "state_unknown"
	}
}

type AsyncMessageHandler = func(err error, n int, mt MessageType)
type AsyncFrameHandler = func(err error, f *Frame)
type ControlCallback = func(mt MessageType, payload []byte)

// Stream is an interface for representing a stateful WebSocket connection
// on the server or client side.
//
// The interface uses the layered stream model. A WebSocket stream object contains
// another stream object, called the "next layer", which it uses to perform IO.
//
// Implementations handle the replies to control frames. Before closing the stream,
// it is important to call Flush or AsyncFlush in order to write any pending
// control frame replies to the underlying stream.
type Stream interface {
	// NextLayer returns the underlying stream object.
	//
	// The returned object is constructed by the Stream and maintained throughout its
	// entire lifetime. All reads and writes will go through the next layer.
	NextLayer() sonic.Stream

	// SupportsDeflate returns true if Deflate compression is supported.
	//
	// https://datatracker.ietf.org/doc/html/rfc7692
	SupportsDeflate() bool

	// SupportsUTF8 returns true if UTF8 validity checks are supported.
	//
	// Implementations should not do UTF8 checking by default. Callers
	// should be able to turn it on when instantiating the Stream.
	SupportsUTF8() bool

	// NextMessage reads the payload of the next message into the supplied buffer.
	// Message fragmentation is automatically handled by the implementation.
	//
	// This call first flushes any pending control frames to the underlying stream.
	//
	// This call blocks until one of the following conditions is true:
	//  - an error occurs while flushing the pending operations
	//  - an error occurs when reading/decoding the message bytes from the underlying stream
	//  - the payload of the message is successfully read into the supplied buffer
	NextMessage([]byte) (mt MessageType, n int, err error)

	// NextFrame reads and returns the next frame.
	//
	// This call first flushes any pending control frames to the underlying stream.
	//
	// This call blocks until one of the following conditions is true:
	//  - an error occurs while flushing the pending operations
	//  - an error occurs when reading/decoding the message bytes from the underlying stream
	//  - a frame is successfully read from the underlying stream
	NextFrame() (*Frame, error)

	// AsyncNextMessage reads the payload of the next message into the supplied buffer
	// asynchronously. Message fragmentation is automatically handled by the implementation.
	//
	// This call first flushes any pending control frames to the underlying stream asynchronously.
	//
	// This call does not block. The provided completion handler is invoked when one of the
	// following happens:
	//  - an error occurs while flushing the pending operations
	//  - an error occurs when reading/decoding the message bytes from the underlying stream
	//  - the payload of the message is successfully read into the supplied buffer
	AsyncNextMessage([]byte, AsyncMessageHandler)

	// AsyncNextFrame reads and returns the next frame asynchronously.
	//
	// This call first flushes any pending control frames to the underlying stream asynchronously.
	//
	// This call does not block. The provided completion handler is invoked when one of the
	// following happens:
	//  - an error occurs while flushing the pending operations
	//  - an error occurs when reading/decoding the message bytes from the underlying stream
	//  - a frame is successfully read from the underlying stream
	AsyncNextFrame(AsyncFrameHandler)

	// WriteFrame writes the supplied frame to the underlying stream.
	//
	// This call first flushes any pending control frames to the underlying stream.
	//
	// This call blocks until one of the following conditions is true:
	//  - an error occurs while flushing the pending operations
	//  - an error occurs during the write
	//  - the frame is successfully written to the underlying stream
	WriteFrame(fr *Frame) error

	// AsyncWriteFrame writes the supplied frame to the underlying stream asynchronously.
	//
	// This call first flushes any pending control frames to the underlying stream asynchronously.
	//
	// This call does not block. The provided completion handler is invoked when one of the
	// following happens:
	//  - an error occurs while flushing the pending operations
	//  - an error occurs during the write
	//  - the frame is successfully written to the underlying stream
	AsyncWriteFrame(fr *Frame, cb func(err error))

	// Write writes the supplied buffer as a single message with the given type
	// to the underlying stream.
	//
	// This call first flushes any pending control frames to the underlying stream.
	//
	// This call blocks until one of the following conditions is true:
	//  - an error occurs while flushing the pending operations
	//  - an error occurs during the write
	//  - the message is successfully written to the underlying stream
	//
	// The message will be written as a single frame. Fragmentation should be handled
	// by the caller through multiple calls to AsyncWriteFrame.
	Write(b []byte, mt MessageType) error

	// AsyncWrite writes the supplied buffer as a single message with the given type
	// to the underlying stream asynchronously.
	//
	// This call first flushes any pending control frames to the underlying stream asynchronously.
	//
	// This call does not block. The provided completion handler is invoked when one of the
	// following happens:
	//  - an error occurs while flushing the pending operations
	//  - an error occurs during the write
	//  - the message is successfully written to the underlying stream
	//
	// The message will be written as a single frame. Fragmentation should be handled
	// by the caller through multiple calls to AsyncWriteFrame.
	AsyncWrite(b []byte, mt MessageType, cb func(err error))

	// Flush writes any pending control frames to the underlying stream.
	//
	// This call blocks.
	Flush() error

	// Flush writes any pending control frames to the underlying stream asynchronously.
	//
	// This call does not block.
	AsyncFlush(cb func(err error))

	// Pending returns the number of currently pending operations.
	Pending() int

	// State returns the state of the WebSocket connection.
	State() StreamState

	// Handshake performs the WebSocket handshake in the client role.
	//
	// The call blocks until one of the following conditions is true:
	//	- the request is sent and the response is received
	//	- an error occurs
	Handshake(addr string) error

	// AsyncHandshake performs the WebSocket handshake asynchronously in the client role.
	//
	// This call does not block. The provided completion handler is called when the request is
	// sent and the response is received or when an error occurs.
	//
	// Regardless of  whether the asynchronous operation completes immediately or not,
	// the handler will not be invoked from within this function. Invocation of the handler
	// will be performed in a manner equivalent to using sonic.Post(...).
	AsyncHandshake(addr string, cb func(error))

	// Accept performs the handshake in the server role.
	//
	// The call blocks until one of the following conditions is true:
	//	- the request is sent and the response is received
	//	- an error occurs
	Accept() error

	// AsyncAccept performs the handshake asynchronously in the server role.
	//
	// This call does not block. The provided completion handler is called when the request is
	// send and the response is received or when an error occurs.
	//
	// Regardless of  whether the asynchronous operation completes immediately or not,
	// the handler will not be invoked from within this function. Invocation of the handler
	// will be performed in a manner equivalent to using sonic.Post(...).
	AsyncAccept(func(error))

	// AsyncClose sends a websocket close control frame asynchronously.
	//
	// This function is used to send a close frame which begins the WebSocket closing handshake.
	// The session ends when both ends of the connection have sent and received a close frame.
	//
	// The handler is called if one of the following conditions is true:
	//	- the close frame is written
	//	- an error occurs
	//
	// After beginning the closing handshake, the program should not write further message data,
	// pings, or pongs. Instead, the program should continue reading message data until
	// an error occurs.
	AsyncClose(cc CloseCode, reason string, cb func(err error))

	// Close sends a websocket close control frame asynchronously.
	//
	// This function is used to send a close frame which begins the WebSocket closing handshake.
	// The session ends when both ends of the connection have sent and received a close frame.
	//
	// The call blocks until one of the following conditions is true:
	//	- the close frame is written
	//	- an error occurs
	//
	// After beginning the closing handshake, the program should not write further message data,
	// pings, or pongs. Instead, the program should continue reading message data until
	// an error occurs.
	Close(cc CloseCode, reason string) error

	// SetControlCallback sets a function that will be invoked when a Ping/Pong/Close is received
	// while reading a message. This callback is not invoked when AsyncNextFrame or NextFrame
	// are called.
	//
	// The caller must not perform any operations on the stream in the provided callback.
	SetControlCallback(ControlCallback)

	// ControlCallback returns the control callback set with SetControlCallback.
	ControlCallback() ControlCallback

	// SetMaxMessageSize sets the maximum size of a message that can be read from
	// or written to a peer.
	//
	//  - If a message exceeds the limit while reading, the connection is
	//    closed abnormally.
	//  - If a message exceeds the limit while writing, the operation is
	//    cancelled.
	SetMaxMessageSize(bytes int)

	// Reset resets the stream state.
	Reset()
}
