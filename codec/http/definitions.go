package http

import (
	"net/http"

	"github.com/talostrading/sonic"
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

	Do(target string, req *http.Request) (*http.Response, error)
	AsyncDo(target string, req *http.Request, cb AsyncResponseHandler)

	State() StreamState

	Proto() string

	NextLayer() sonic.Stream
}
