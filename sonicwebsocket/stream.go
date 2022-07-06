package sonicwebsocket

import "github.com/talostrading/sonic"

type StreamState uint8

const (
	StateHandshake StreamState = iota
	StateOpen
	StateClosing
	StateClosed
)

func (s StreamState) String() string {
	switch s {
	case StateHandshake:
		return "state_handshake"
	case StateOpen:
		return "state_open"
	case StateClosing:
		return "state_closing"
	case StateClosed:
		return "state_closed"
	default:
		return "state_unknown"
	}
}

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
	Read(b []byte) (n int, err error)

	// ReadSome reads some part of a message into b.
	//
	// The chunk of the message should come from a valid WebSocket frame.
	//
	// If b cannot hold the message, ErrPayloadTooBig is returned
	ReadSome(b []byte) (n int, err error)

	// AsyncRead reads a complete message into b asynchronously.
	//
	// If b cannot hold the message, ErrPayloadTooBig is provided in the handler invocation.
	AsyncRead(b []byte, cb sonic.AsyncCallback)

	// AsyncReadSome reads some part of a message into b asynchronously.
	//
	// The chunk of the message should come from a valid WebSocket frame.
	//
	// If b cannot hold the message, ErrPayloadTooBig is provided in the handler invocation.
	AsyncReadSome(b []byte, cb sonic.AsyncCallback)

	// Write writes a complete message from b.
	//
	// The message can be composed of one or several frames.
	Write(b []byte) (n int, err error)

	// WriteSome writes some part of a message from b.
	//
	// A caller should expect this function to write frames with the provided buffer
	// as payload. If the payload is too big, the caller has the option of breaking
	// up the message into multiple frames through multiple calls to AsyncWriteSome.
	// In this case, fin should be set to false on all calls but the last one, where
	// it should be set to true.
	WriteSome(fin bool, b []byte) (n int, err error)

	// AsyncWrite writes a complete message from b asynchronously.
	//
	// The message can be composed of one or several frames.
	AsyncWrite(b []byte, cb sonic.AsyncCallback)

	// AsyncWriteSome writes some part of a message from b asynchronously.
	//
	// A caller should expect this function to write frames with the provided buffer
	// as payload. If the payload is too big, the caller has the option of breaking
	// up the message into multiple frames through multiple calls to AsyncWriteSome.
	// In this case, fin should be set to false on all calls but the last one, where
	// it should be set to true.
	AsyncWriteSome(fin bool, b []byte, cb sonic.AsyncCallback)

	// SetReadLimit sets the maximum read size. If 0, the max size is used.
	SetReadLimit(uint64)

	// State returns the state of the WebSocket connection.
	State() StreamState

	// GotText returns true if the latest message data indicates text.
	//
	// This function informs the caller of whether the last
	// received message frame represents a message with the
	// text opcode.
	//
	// If there is no last message frame, the return value
	// is undefined.
	GotText() bool

	// GotBinary returns true if the latest message data indicates binary.
	//
	// This function informs the caller of whether the last
	// received message frame represents a message with the
	// binary opcode.
	//
	// If there is no last message frame, the return value
	// is undefined.
	GotBinary() bool

	// IsMessageDone returns true if the last completed read finished the current message.
	IsMessageDone() bool

	// SentBinary returns true if the last sent frame was binary.
	SentBinary() bool

	// SentText returns true if the last sent frame was text.
	SentText() bool

	// SendBinary TODO doc
	SendBinary(bool)

	// SendText TODO doc
	SendText(bool)

	// SetControlCallback sets a callback to be invoked on each incoming control frame.
	//
	// Sets the callback to be invoked whenever a ping, pong, or close control frame
	// is received during a call to one of the following functions:
	//	- AsyncRead
	//	- AsyncReadAll // TODO maybe change stuff to have AsyncReadSome and AsyncRead then will read completely
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
	// will be performed in a manner equivalent to using sonic.Dispatch(...).
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
	// will be performed in a manner equivalent to using sonic.Dispatch(...).
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

	// Closed indicates whether the underlying connection is closed.
	Closed() bool

	// Ping sends a websocket ping control frame.
	//
	// The call blocks until one of the following conditions is true:
	//  - the ping frame is written
	//  - an error occurs
	Ping([]byte) error

	// AsyncPing sends a websocket ping control frame asynchronously.
	//
	// This call always returns immediately. The asynchronous operation will continue until
	// one of the following conditions is true:
	//	- the ping frame finishes sending
	//	- an error occurs
	AsyncPing([]byte, func(error))

	// Pong sends a websocket pong control frame.
	//
	// The call blocks until one of the following conditions is true:
	//  - the pong frame is written
	//  - an error occurs
	Pong([]byte) error

	// AsyncPong sends a websocket pong control frame asynchronously.
	//
	// This call always returns immediately. The asynchronous operation will continue until
	// one of the following conditions is true:
	//	- the pong frame finishes sending
	//	- an error occurs
	AsyncPong([]byte, func(error))
}
