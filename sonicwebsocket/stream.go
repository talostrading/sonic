package sonicwebsocket

import "github.com/talostrading/sonic"

type StreamState uint8

const (
	// The connection is being established.
	StateHandshake StreamState = iota

	// The connection is active.
	StateActive

	// We initiated a close handshake.
	StateClosedByUs

	// The peer initiated a close handshake.
	StateClosedByPeer

	// The peer replied to our close handshake.
	StateCloseAcked

	// The connection does not exist anymore.
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

type AsyncCallback func(err error, n int, t MessageType)

// Stream is an interface for representing a stateful WebSocket connection
// on the server or client side.
//
// The interface uses the layered stream model. A WebSocket stream object contains
// another stream object, called the "next layer", which it uses to perform IO.
//
// The smallest unit of data for a WebSocket stream is a frame. A message may be composed
// from multiple frames. Clients will never get a partial frame after executing a read.
// They might get partial messages, if a message is fragmented, after a call
// to ReadSome(...) or AsyncReadSome(...).
type Stream interface {
	// NextLayer returns the underlying stream object.
	//
	// The returned object is constructed by the Stream and maintained throughout its
	// entire lifetime. All reads and writes will go through the next layer.
	NextLayer() sonic.Stream

	// https://datatracker.ietf.org/doc/html/rfc7692
	DeflateSupported() bool

	// Reads reads a complete message into b.
	//
	// If b cannot hold the message, ErrPayloadTooBig is returned.
	Read(b []byte) (t MessageType, n int, err error)

	// ReadSome reads some part of a message into b.
	//
	// The chunk of the message should come from a valid WebSocket frame.
	//
	// If b cannot hold the message, ErrPayloadTooBig is returned
	ReadSome(b []byte) (t MessageType, n int, err error)

	// AsyncRead reads a complete message into b asynchronously.
	//
	// If b cannot hold the message, ErrPayloadTooBig is provided in the handler invocation.
	AsyncRead(b []byte, cb AsyncCallback)

	// AsyncReadSome reads some part of a message into b asynchronously.
	//
	// The message chunk is a valid WebSocket frame.
	//
	// If b cannot hold the message, ErrPayloadTooBig is provided in the handler invocation.
	AsyncReadSome(b []byte, cb AsyncCallback)

	// WriteFrame writes the given frame.
	WriteFrame(fr *Frame) error

	// AsyncWriteFrame writes the given frame asynchronously.
	AsyncWriteFrame(fr *Frame, cb func(error))

	AsyncFlush(cb func(err error))

	Flush() error

	// SetMaxMessageSize sets the maximum read message size. If 0, the default MaxMessageSize is used.
	SetMaxMessageSize(uint64)

	// State returns the state of the WebSocket connection.
	State() StreamState

	// IsMessageDone returns true if the last completed read finished the current message.
	IsMessageDone() bool

	// SetControlCallback sets a callback to be invoked on each incoming control frame.
	//
	// Sets the callback to be invoked whenever a ping, pong, or close control frame
	// is received during a call to one of the following functions:
	//	- AsyncRead
	//	- AsyncReadSome
	SetControlCallback(AsyncControlCallback)

	// ControlCallback returns the set control callback invoked on each incoming control frame.
	//
	// If not control callback is set, nil is returned.
	ControlCallback() AsyncControlCallback

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
