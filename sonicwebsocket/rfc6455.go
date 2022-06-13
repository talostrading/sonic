package sonicwebsocket

import "net/http"

// IsUpgrade returns true if the HTTP request is a WebSocket upgrade.
//
// This function returns true when the passed HTTP request indicates
// a WebSocket Upgrade. It does not validate the contents of the fields.
func IsUpgrade(req *http.Request) bool { // TODO use this
	// TODO
	return false
}

// The max size of the ping/pong control frame payload.
const PingPongPayloadSize = 125

// The type representing the reason string in a close frame.
type ReasonString [123]byte

// Close status codes.
//
// These codes accompany close frames.
type CloseCode uint16

const (
	// Normal signifies normal closure; the connection successfully
	// completed whatever purpose for which it was created.
	Normal CloseCode = 1000

	// GoingaAway means endpoint is going away, either because of a
	// server failure or because the browser is navigating away from
	// the page that opened the connection.
	GoingAway = 1001

	// ProtocolError means the endpoint is terminating the connection
	// due to a protocol error.
	ProtocolError = 1002

	// UnknownData means the connection is being terminated because
	// the endpoint received data of a type it cannot accept (for example,
	// a text-only endpoint received binary data).
	UnknownData = 1003

	// BadPayload means the endpoint is terminating the connection because
	// a message was received that contained inconsistent data
	// (e.g., non-UTF-8 data within a text message).
	BadPayload = 1007

	// PolicyError means the endpoint is terminating the connection because
	// it received a message that violates its policy. This is a generic status
	// code, used when codes 1003 and 1009 are not suitable.
	PolicyError = 1008

	// TooBig means the endpoint is terminating the connection because a data
	// frame was received that is too large.
	TooBig = 1009

	// NeedsExtension means the client is terminating the connection because it
	// expected the server to negotiate one or more extensions, but the server didn't.
	NeedsExtension = 1010

	// InternalError means the server is terminating the connection because it
	// encountered an unexpected condition that prevented it from fulfilling the request.
	InternalError = 1011

	// ServiceRestart means the server is terminating the connection because it is restarting.
	ServiceRestart = 1012

	// TryAgainLater means the server is terminating the connection due to a temporary
	// condition, e.g. it is overloaded and is casting off some of its clients.
	TryAgainLater = 1013

	// -------------------------------------
	// The following are illegal on the wire
	// -------------------------------------

	// None is used internally to mean "no error"
	// This code is reserved and may not be sent.
	None = 0

	// NoStatus means no status code was provided even though one was expected.
	// This code is reserved and may not be sent.
	NoStatus = 1005

	// Abnormal means the connection was closed without receiving a close frame.
	// This code is reserved and may not be sent.
	Abnormal = 1006

	// Reserved1 is reserved for future use by the WebSocket standard.
	// This code is reserved and may not be sent.
	Reserved1 = 1004

	// Reserved2 is reserved for future use by the WebSocket standard.
	// This code is reserved and may not be sent.
	Reserved2 = 1014

	// Reserved3 is reserved for future use by the WebSocket standard.
	// This code is reserved and may not be sent.
	Reserved3 = 1015
)
