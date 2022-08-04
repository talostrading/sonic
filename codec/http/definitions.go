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
	StateDisconnecting
	StateReconnecting
	StateConnected
)

func (s StreamState) String() string {
	switch s {
	case StateDisconnected:
		return "state_disconnected"
	case StateDisconnecting:
		return "state_disconnecting"
	case StateReconnecting:
		return "state_reconnecting"
	case StateConnected:
		return "state_connected"
	default:
		return "state_unknown"
	}
}

type AsyncResponseHandler func(error, *http.Response)

type Stream interface {
	Connect(addr string) error
	AsyncConnect(addr string, cb func(err error))

	Do(target string, req *http.Request) (*http.Response, error)
	AsyncDo(target string, req *http.Request, cb AsyncResponseHandler)

	State() StreamState

	Proto() string

	NextLayer() sonic.Stream

	Close()
}
