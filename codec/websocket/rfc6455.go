// Based on https://datatracker.ietf.org/doc/html/rfc6455
package websocket

import (
	"encoding/binary"
	"net/http"
	"strings"
)

// ---------------------------------------------------
// Framing -------------------------------------------
// ---------------------------------------------------
// Based on https://datatracker.ietf.org/doc/html/rfc6455#section-5.2

const (
	MaxControlFramePayloadLength = 125
	frameMaxHeaderLength         = 14 // 14 bytes max for the header of a frame i.e. everything without the payload
)

const (
	// Mandatory, 2 bytes:
	// byte 1: |fin(1)|rsv1(1)|rsv2(1)|rsv3(1)|opcode(4)|
	// byte 2: |is masked(1)|payload length(7)|
	frameHeaderLength = 2

	bitFIN        = byte(1 << 7)
	bitRSV1       = byte(1 << 6)
	bitRSV2       = byte(1 << 5)
	bitRSV3       = byte(1 << 4)
	bitmaskOpcode = byte(1<<4 - 1)

	bitIsMasked          = byte(1 << 7)
	bitmaskPayloadLength = byte(1<<7 - 1)

	// Optional, max 8 bytes. If |payload length(7)| above is <= 125, then 0: the payload length is in
	// |payload length(7)|. If 126, then the payload length is in the following 2 bytes. If 127, in the following 8
	// bytes.

	// Optional, max 4 bytes. If |is masked(1)| above is set, then the following 4 bytes are the mask. Otherwise, the
	// frame is not masked and the mask is not included.
	//
	// All frames sent from the client to the server are masked by a 32-bit value. Frames sent from the server to the
	// client are unmasked.
	frameMaskLength = 4
)

type Opcode byte

const (
	OpcodeContinuation Opcode = 0
	OpcodeText         Opcode = 1
	OpcodeBinary       Opcode = 2
	OpcodeClose        Opcode = 8
	OpcodePing         Opcode = 9
	OpcodePong         Opcode = 10
)

func (c Opcode) IsContinuation() bool { return c == OpcodeContinuation }
func (c Opcode) IsText() bool         { return c == OpcodeText }
func (c Opcode) IsBinary() bool       { return c == OpcodeBinary }
func (c Opcode) IsClose() bool        { return c == OpcodeClose }
func (c Opcode) IsPing() bool         { return c == OpcodePing }
func (c Opcode) IsPong() bool         { return c == OpcodePong }

func (c Opcode) IsReserved() bool {
	return c != OpcodeContinuation &&
		c != OpcodeText &&
		c != OpcodeBinary &&
		c != OpcodeClose &&
		c != OpcodePing &&
		c != OpcodePong
}

func (c Opcode) IsControl() bool {
	return c.IsPing() || c.IsPong() || c.IsClose()
}

func (c Opcode) String() string {
	switch c {
	case OpcodeContinuation:
		return "continuation"
	case OpcodeText:
		return "text"
	case OpcodeBinary:
		return "binary"
	case OpcodeClose:
		return "close"
	case OpcodePing:
		return "ping"
	case OpcodePong:
		return "pong"
	default:
		return "unknown"
	}
}

// ---------------------------------------------------
// Handshake -----------------------------------------
// ---------------------------------------------------

// Used when constructing the server's Sec-WebSocket-Accept key based on the client's Sec-WebSocket-Key.
var GUID = []byte("258EAFA5-E914-47DA-95CA-C5AB0DC85B11")

func IsUpgradeReq(req *http.Request) bool {
	return strings.EqualFold(req.Header.Get("Upgrade"), "websocket")
}

func IsUpgradeRes(res *http.Response) bool {
	return res.StatusCode == 101 &&
		strings.EqualFold(res.Header.Get("Upgrade"), "websocket")
}

// ---------------------------------------------------
// Closing -------------------------------------------
// ---------------------------------------------------

// Close status codes that accompany close frames.
type CloseCode uint16

const (
	// CloseNormal signifies normal closure; the connection successfully completed whatever purpose for which it was
	// created.
	CloseNormal CloseCode = 1000

	// GoingAway means endpoint is going away, either because of a server failure or because the browser is navigating
	// away from the page that opened the connection.
	CloseGoingAway CloseCode = 1001

	// CloseProtocolError means the endpoint is terminating the connection due to a protocol error.
	CloseProtocolError CloseCode = 1002

	// CloseUnknownData means the connection is being terminated because the endpoint received data of a type it cannot
	// accept (for example, a text-only endpoint received binary data).
	CloseUnknownData CloseCode = 1003

	// CloseBadPayload means the endpoint is terminating the connection because a message was received that contained
	// inconsistent data (e.g., non-UTF-8 data within a text message).
	CloseBadPayload CloseCode = 1007

	// ClosePolicyError means the endpoint is terminating the connection because it received a message that violates its
	// policy. This is a generic status code, used when codes 1003 and 1009 are not suitable.
	ClosePolicyError CloseCode = 1008

	// CloseTooBig means the endpoint is terminating the connection because a data frame was received that is too large.
	CloseTooBig CloseCode = 1009

	// CloseNeedsExtension means the client is terminating the connection because it expected the server to negotiate
	// one or more extensions, but the server didn't.
	CloseNeedsExtension CloseCode = 1010

	// CloseInternalError means the server is terminating the connection because it encountered an unexpected condition
	// that prevented it from fulfilling the request.
	CloseInternalError CloseCode = 1011

	// CloseServiceRestart means the server is terminating the connection because it is restarting.
	CloseServiceRestart CloseCode = 1012

	// CloseTryAgainLater means the server is terminating the connection due to a temporary condition, e.g. it is
	// overloaded and is casting off some of its clients.
	CloseTryAgainLater CloseCode = 1013

	// -------------------------------------
	// The following are illegal on the wire
	// -------------------------------------

	// CloseNone is used internally to mean "no error" This code is reserved and may not be sent.
	CloseNone CloseCode = 0

	// CloseNoStatus means no status code was provided in the close frame sent by the peer, even though one was
	// expected.  This code is reserved for internal use and may not be sent in-between peers.
	CloseNoStatus CloseCode = 1005

	// CloseAbnormal means the connection was closed without receiving a close frame. This code is reserved and may not
	// be sent.
	CloseAbnormal CloseCode = 1006

	// CloseReserved1 is reserved for future use by the WebSocket standard. This code is reserved and may not be sent.
	CloseReserved1 CloseCode = 1004

	// CloseReserved2 is reserved for future use by the WebSocket standard. This code is reserved and may not be sent.
	CloseReserved2 CloseCode = 1014

	// CloseReserved3 is reserved for future use by the WebSocket standard. This code is reserved and may not be sent.
	CloseReserved3 CloseCode = 1015

	// CloseReserved4 is reserved for future use by the WebSocket standard. This code is reserved and may not be sent.
	CloseReserved4 CloseCode = 1016

	CloseReservedForFuture CloseCode = 1004
)

func ValidCloseCode(closeCode CloseCode) bool {
	if closeCode == CloseNormal ||
		closeCode == CloseGoingAway ||
		closeCode == CloseProtocolError ||
		closeCode == CloseUnknownData ||
		closeCode == CloseBadPayload ||
		closeCode == ClosePolicyError ||
		closeCode == CloseTooBig ||
		closeCode == CloseNeedsExtension ||
		closeCode == CloseInternalError ||
		closeCode == CloseServiceRestart ||
		closeCode == CloseTryAgainLater ||
		(closeCode >= 3000 && closeCode <= 4999) {
		return true
	}
	return false
}

func EncodeCloseCode(cc CloseCode) []byte {
	b := make([]byte, 2)
	binary.BigEndian.PutUint16(b, uint16(cc))
	return b
}

func DecodeCloseCode(b []byte) CloseCode {
	return CloseCode(binary.BigEndian.Uint16(b[:2]))
}
