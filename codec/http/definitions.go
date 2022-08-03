package http

import "net/http"

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

type StreamState uint8

const (
	StateDisconnected StreamState = iota
	StateConnected
)

type AsyncResponseHandler func(error, *http.Response)

type Stream interface {
	Connect(addr string) error
	AsyncConnect(addr string, cb func(err error))

	Do(*http.Request) (*http.Response, error)
	AsyncDo(*http.Request, AsyncResponseHandler)

	State() StreamState

	Proto() string

	Close()
}
