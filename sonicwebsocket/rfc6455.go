package sonicwebsocket

import (
	"net/http"
	"strings"
)

// GUID is used when constructing the Sec-WebSocket-Accept key based on Sec-WebSocket-Key.
var GUID = []byte("258EAFA5-E914-47DA-95CA-C5AB0DC85B11")

// IsUpgrade returns true if the HTTP request is a WebSocket upgrade.
//
// This function returns true when the passed HTTP request indicates
// a WebSocket Upgrade. It does not validate the contents of the fields.
func IsUpgrade(req *http.Request) bool {
	return strings.EqualFold(req.Header.Get("Upgrade"), "websocket")
}

const (
	finBit  = byte(1 << 7)
	rsv1Bit = byte(1 << 6)
	rsv2Bit = byte(1 << 5)
	rsv3Bit = byte(1 << 4)
	maskBit = byte(1 << 7)
)

// The max size of the ping/pong control frame payload.
const MaxControlFramePayloadSize = 125

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
	GoingAway CloseCode = 1001

	// ProtocolError means the endpoint is terminating the connection
	// due to a protocol error.
	ProtocolError CloseCode = 1002

	// UnknownData means the connection is being terminated because
	// the endpoint received data of a type it cannot accept (for example,
	// a text-only endpoint received binary data).
	UnknownData CloseCode = 1003

	// BadPayload means the endpoint is terminating the connection because
	// a message was received that contained inconsistent data
	// (e.g., non-UTF-8 data within a text message).
	BadPayload CloseCode = 1007

	// PolicyError means the endpoint is terminating the connection because
	// it received a message that violates its policy. This is a generic status
	// code, used when codes 1003 and 1009 are not suitable.
	PolicyError CloseCode = 1008

	// TooBig means the endpoint is terminating the connection because a data
	// frame was received that is too large.
	TooBig CloseCode = 1009

	// NeedsExtension means the client is terminating the connection because it
	// expected the server to negotiate one or more extensions, but the server didn't.
	NeedsExtension CloseCode = 1010

	// InternalError means the server is terminating the connection because it
	// encountered an unexpected condition that prevented it from fulfilling the request.
	InternalError CloseCode = 1011

	// ServiceRestart means the server is terminating the connection because it is restarting.
	ServiceRestart CloseCode = 1012

	// TryAgainLater means the server is terminating the connection due to a temporary
	// condition, e.g. it is overloaded and is casting off some of its clients.
	TryAgainLater CloseCode = 1013

	// -------------------------------------
	// The following are illegal on the wire
	// -------------------------------------

	// None is used internally to mean "no error"
	// This code is reserved and may not be sent.
	None CloseCode = 0

	// NoStatus means no status code was provided even though one was expected.
	// This code is reserved and may not be sent.
	NoStatus CloseCode = 1005

	// Abnormal means the connection was closed without receiving a close frame.
	// This code is reserved and may not be sent.
	Abnormal CloseCode = 1006

	// Reserved1 is reserved for future use by the WebSocket standard.
	// This code is reserved and may not be sent.
	Reserved1 CloseCode = 1004

	// Reserved2 is reserved for future use by the WebSocket standard.
	// This code is reserved and may not be sent.
	Reserved2 CloseCode = 1014

	// Reserved3 is reserved for future use by the WebSocket standard.
	// This code is reserved and may not be sent.
	Reserved3 CloseCode = 1015
)

type Opcode uint8

// No `iota` here for clarity.
const (
	OpcodeContinuation Opcode = 0x00
	OpcodeText         Opcode = 0x01
	OpcodeBinary       Opcode = 0x02
	OpcodeRsv3         Opcode = 0x03
	OpcodeRsv4         Opcode = 0x04
	OpcodeRsv5         Opcode = 0x05
	OpcodeRsv6         Opcode = 0x06
	OpcodeRsv7         Opcode = 0x07
	OpcodeClose        Opcode = 0x08
	OpcodePing         Opcode = 0x09
	OpcodePong         Opcode = 0x0A
	OpcodeCrsvb        Opcode = 0x0B
	OpcodeCrsvc        Opcode = 0x0C
	OpcodeCrsvd        Opcode = 0x0D
	OpcodeCrsve        Opcode = 0x0E
	OpcodeCrsvf        Opcode = 0x0F
)

func IsReserved(op Opcode) bool {
	return (op >= OpcodeRsv3 && op <= OpcodeRsv7) || (op >= OpcodeCrsvb && op <= OpcodeCrsvf)
}

func (c Opcode) String() string {
	switch c {
	case OpcodeContinuation:
		return "continuation"
	case OpcodeText:
		return "text"
	case OpcodeBinary:
		return "binary"
	case OpcodeRsv3:
		return "rsv3"
	case OpcodeRsv4:
		return "rsv4"
	case OpcodeRsv5:
		return "rsv5"
	case OpcodeRsv6:
		return "rsv6"
	case OpcodeRsv7:
		return "rsv7"
	case OpcodeClose:
		return "close"
	case OpcodePing:
		return "ping"
	case OpcodePong:
		return "pong"
	case OpcodeCrsvb:
		return "crsvb"
	case OpcodeCrsvc:
		return "crsvc"
	case OpcodeCrsvd:
		return "crsvd"
	case OpcodeCrsve:
		return "crsve"
	case OpcodeCrsvf:
		return "crsvf"
	default:
		return "unknown"
	}
}
