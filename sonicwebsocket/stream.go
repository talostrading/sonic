package sonicwebsocket

import "github.com/talostrading/sonic"

type StreamState uint8

const (
	StateHandshake StreamState = iota
	StateOpen
	StateClosing
	StateClosed
)

type StateChangeCallback func(err error, state StreamState)

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

	// TODO: DeflateSupported (https://datatracker.ietf.org/doc/html/rfc7692)

	// Reads reads a complete message into b.
	//
	// If b cannot hold the message, TODO error is returned
	Read(b []byte) error

	// ReadSome reads some part of a message into b.
	ReadSome(b []byte) error

	// AsyncRead reads a complete message asynchronously.
	AsyncRead(b []byte, cb sonic.AsyncCallback)

	// AsyncReadSome reads some part of a message asynchronously.
	AsyncReadSome(b []byte, cb sonic.AsyncCallback)

	// Write writes a complete message.
	Write(b []byte) error

	// WriteSome writes some message data.
	WriteSome(fin bool, b []byte) error

	// AsyncWrite writes a complete message asynchronously.
	AsyncWrite(b []byte, cb sonic.AsyncCallback)

	// AsyncWriteSome writes some message data asynchronously.
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

	// Close sends a websocket close control frame.
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
	Close(*CloseReason) error

	// AsyncClose sends a websocket close control frame asynchronously.
	//
	// This function is used to send a close frame which begins the WebSocket closing handshake.
	// The session ends when both ends of the connection have sent and received a close frame.
	//
	// This call always returns immediately. The asynchronous operation will continue until
	// one of the following conditions is true:
	//	- the close frame finishes sending
	//	- an error occurs
	//
	// After beginning the closing handshake, the program should not write further message data,
	// pings, or pongs. Instead, the program should continue reading message data until
	// an error occurs.
	AsyncClose(*CloseReason, func(error))

	// Ping sends a websocket ping control frame.
	//
	// The call blocks until one of the following conditions is true:
	//  - the ping frame is written
	//  - an error occurs
	Ping(PingPongPayload) error

	// AsyncPing sends a websocket ping control frame asynchronously.
	//
	// This call always returns immediately. The asynchronous operation will continue until
	// one of the following conditions is true:
	//	- the ping frame finishes sending
	//	- an error occurs
	AsyncPing(PingPongPayload, func(error))

	// Pong sends a websocket pong control frame.
	//
	// The call blocks until one of the following conditions is true:
	//  - the pong frame is written
	//  - an error occurs
	Pong(PingPongPayload) error

	// AsyncPong sends a websocket pong control frame asynchronously.
	//
	// This call always returns immediately. The asynchronous operation will continue until
	// one of the following conditions is true:
	//	- the pong frame finishes sending
	//	- an error occurs
	AsyncPong(PingPongPayload, func(error))

	Options() *Options
}
