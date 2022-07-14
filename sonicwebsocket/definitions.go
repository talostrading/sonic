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

type AsyncControlCallback func(Opcode, []byte)

const (
	FrameHeaderSize int = 10
	FrameMaskSize   int = 4

	DefaultPayloadSize  int = 4096
	MaxFramePayloadSize int = 1024 * 512
	MaxPending          int = 16392

	DefaultFrameSize = FrameHeaderSize + FrameMaskSize + DefaultPayloadSize
)

type MessageType uint8

const (
	TypeText   = MessageType(OpcodeText)
	TypeBinary = MessageType(OpcodeBinary)
	TypeClose  = MessageType(OpcodeClose)
	TypePing   = MessageType(OpcodePing)
	TypePong   = MessageType(OpcodePong)

	// Sent when failing to read a complete frame.
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
	case TypeNone:
		return "type_none"
	default:
		return "type_unknown"
	}
}
