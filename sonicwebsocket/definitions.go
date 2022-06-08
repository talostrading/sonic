package sonicwebsocket

type AsyncControlCallback func(FrameType, []byte)

type FrameType uint8

const (
	Close FrameType = iota
	Ping
	Pong
)

type Opcode uint8

// No `iota` here for clarity.
const (
	OpcodeContinuation Opcode = 0x0
	OpcodeText         Opcode = 0x1
	OpcodeBinary       Opcode = 0x2
	OpcodeRsv3         Opcode = 0x3
	OpcodeRsv4         Opcode = 0x4
	OpcodeRsv5         Opcode = 0x5
	OpcodeRsv6         Opcode = 0x6
	OpcodeRsv7         Opcode = 0x7
	OpcodeClose        Opcode = 0x8
	OpcodePing         Opcode = 0x9
	OpcodePong         Opcode = 0xA
	OpcodeCrsvb        Opcode = 0xB
	OpcodeCrsvc        Opcode = 0xC
	OpcodeCrsvd        Opcode = 0xD
	OpcodeCrsve        Opcode = 0xE
	OpcodeCrsvf        Opcode = 0xF
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

const (
	frameHeaderSize uint64 = 10 // 10B
	frameMaskSize   uint64 = 4  // 4B

	DefaultPayloadSize uint64 = 4096    // 4KB
	MaxPayloadSize     uint64 = 1 << 32 // 4GB
)

const (
	finBit  = byte(1 << 7)
	rsv1Bit = byte(1 << 6)
	rsv2Bit = byte(1 << 5)
	rsv3Bit = byte(1 << 4)
	maskBit = byte(1 << 7)
)
