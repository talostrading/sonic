package sonicwebsocket

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
		return "role_server"
	default:
		return "role_unknown"
	}
}

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

const (
	frameHeaderSize uint64 = 10 // 10B
	frameMaskSize   uint64 = 4  // 4B

	DefaultPayloadSize uint64 = 4096    // 4KB
	MaxPayloadSize     uint64 = 1 << 32 // 4GB
	MaxPending         uint64 = 8196    // 8KB
)

const (
	finBit  = byte(1 << 7)
	rsv1Bit = byte(1 << 6)
	rsv2Bit = byte(1 << 5)
	rsv3Bit = byte(1 << 4)
	maskBit = byte(1 << 7)
)
