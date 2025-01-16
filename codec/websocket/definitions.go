package websocket

import (
	"net/http"
	"time"
)

const (
	DefaultMaxMessageSize = 1024 * 512
	CloseTimeout          = 5 * time.Second
	DialTimeout           = 5 * time.Second
)

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
		return "role_client"
	default:
		return "role_unknown"
	}
}

type MessageType byte

const (
	TypeText   = MessageType(OpcodeText)
	TypeBinary = MessageType(OpcodeBinary)
	TypeClose  = MessageType(OpcodeClose)
	TypePing   = MessageType(OpcodePing)
	TypePong   = MessageType(OpcodePong)

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
	default:
		return "type_unknown"
	}
}

type StreamState uint8

const (
	// Start state. Handshake is ongoing.
	StateHandshake StreamState = iota

	// Intermediate state. Connection is active, can read/write/close.
	StateActive

	// Intermediate state. We initiated the closing handshake and are waiting for a reply from the peer.
	StateClosedByUs

	// Terminal state. The peer initiated the closing handshake, we received a close frame and immediately replied.
	StateClosedByPeer

	// Terminal state. The peer replied to our closing handshake. Can only end up here from StateClosedByUs.
	StateCloseAcked

	// Terminal state. The connection is closed or some error occurred which rendered the stream unusable.
	StateTerminated
)

func (s StreamState) String() string {
	switch s {
	case StateHandshake:
		return "state_handshake"
	case StateActive:
		return "state_active"
	case StateClosedByUs:
		return "state_closed_by_us"
	case StateClosedByPeer:
		return "state_closed_by_peer"
	case StateCloseAcked:
		return "state_closed_acked"
	case StateTerminated:
		return "state_terminated"
	default:
		return "state_unknown"
	}
}

type AsyncMessageCallback = func(err error, n int, messageType MessageType)
type AsyncMessageDirectCallback = func(err error, messageType MessageType, payloads []byte)
type AsyncFrameCallback = func(err error, f Frame)
type ControlCallback = func(messageType MessageType, payload []byte)
type UpgradeRequestCallback = func(req *http.Request)
type UpgradeResponseCallback = func(res *http.Response)

type Header struct {
	Key          string
	Values       []string
	CanonicalKey bool
}

func ExtraHeader(canonicalKey bool, key string, values ...string) Header {
	return Header{
		Key:          key,
		Values:       values,
		CanonicalKey: canonicalKey,
	}
}
