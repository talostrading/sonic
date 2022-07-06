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
	frameHeaderSize uint64 = 10 // 10B
	frameMaskSize   uint64 = 4  // 4B

	DefaultPayloadSize uint64 = 4096    // 4KB
	MaxPayloadSize     uint64 = 1 << 32 // 4GB
	MaxPending         uint64 = 8196    // 8KB

	DefaultFrameSize = frameHeaderSize + frameMaskSize + DefaultPayloadSize
)

// operation defines the common interface that all websocket operations
// such as read/write/close etc. should implement.
type operation interface {
	ID() int
}
