package websocket

import (
	"errors"

	"github.com/talostrading/sonic"
)

var (
	ErrPayloadTooBig       = errors.New("frame payload too big")
	ErrWrongHandshakeRole  = errors.New("wrong role when initiating/accepting the handshake")
	ErrCannotUpgrade       = errors.New("cannot upgrade connection to WebSocket")
	ErrInvalidControlFrame = errors.New("invalid control frame")
	ErrSendAfterClose      = errors.New("sending on a closed stream")
)

const (
	MaxPayloadLen = 1024 * 512
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

// Stream is an interface for representing a stateful WebSocket connection
// on the server or client side.
//
// The interface uses the layered stream model. A WebSocket stream object contains
// another stream object, called the "next layer", which it uses to perform IO.
type Stream interface {
	// NextLayer returns the underlying stream object.
	//
	// The returned object is constructed by the Stream and maintained throughout its
	// entire lifetime. All reads and writes will go through the next layer.
	NextLayer() sonic.Stream

	// https://datatracker.ietf.org/doc/html/rfc7692
	DeflateSupported() bool

	NextMessage([]byte) (mt MessageType, n int, err error)
	NextFrame() (*Frame, error)

	AsyncNextMessage([]byte, AsyncMessageHandler)
	AsyncNextFrame(AsyncFrameHandler)

	WriteFrame(fr *Frame) error
	AsyncWriteFrame(fr *Frame, cb func(err error))

	// Flush writes any pending operations such as Pong or Close.
	Flush() error

	// Flush writes any pending operations such as Pong or Close asynchronously.
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
	// send and the response is received or when an error occurs.
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
}
