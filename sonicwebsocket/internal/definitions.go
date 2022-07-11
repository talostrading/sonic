package internal

type Operation int

const (
	OpNoop Operation = iota
	OpAccept
	OpClose
	OpHandshake
	OpPing
	OpPong
	OpRead
	OpWrite
)

func (op Operation) String() string {
	switch op {
	case OpNoop:
		return "noop"
	case OpAccept:
		return "accept"
	case OpClose:
		return "close"
	case OpHandshake:
		return "handshake"
	case OpPing:
		return "ping"
	case OpPong:
		return "pong"
	case OpRead:
		return "read"
	case OpWrite:
		return "write"
	default:
		return "unknown"
	}
}
