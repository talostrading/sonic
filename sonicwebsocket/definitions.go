package sonicwebsocket

import "errors"

type Opcode uint8

const (
	OpcodeContinuation Opcode = 0x0
	OpcodeText         Opcode = 0x1
	OpcodeBinary       Opcode = 0x2
	OpcodeClose        Opcode = 0x8
	OpcodePing         Opcode = 0x9
	OpcodePong         Opcode = 0xA
)

type Status uint16

const (
	StatusNone             Status = 1000
	StatusGoAway           Status = 1001
	StatusProtocolError    Status = 1002
	StatusNotAcceptable    Status = 1003
	StatusReserved         Status = 1004
	StatusNotPresent       Status = 1005
	StatusClosedAbnormally Status = 1006
	StatusInconsistentType Status = 1007
	StatusViolation        Status = 1008
	StatusTooBig           Status = 1009
	StatusExtensionNeeded  Status = 1010
	StatusUnexpected       Status = 1011
	StatusFailedTLS        Status = 1015
)

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

const (
	finBit  = byte(1 << 7)
	rsv1Bit = byte(1 << 6)
	rsv2Bit = byte(1 << 5)
	rsv3Bit = byte(1 << 4)
	maskBit = byte(1 << 7)
)

var (
	ErrCannotUpgrade         = errors.New("cannot upgrade to websocket")
	ErrReadingHeader         = errors.New("could not read header")
	ErrReadingExtendedLength = errors.New("could not read extended length")
	ErrReadingMask           = errors.New("could not read mask")
)
