package http

import (
	"bytes"
	"fmt"
	"github.com/talostrading/sonic"
	"io"
	"net/http"
)

type decodeState uint8

const (
	stateFirstLine decodeState = iota
	stateHeader
	stateBody
	stateDone
)

type Proto string

const (
	headerDelim = ": "

	ProtoHttp11 = "HTTP/1.1"
)

func ParseProtoFromBytes(b []byte) (Proto, error) {
	if bytes.Equal([]byte(ProtoHttp11), b) {
		return ProtoHttp11, nil
	}
	return "", fmt.Errorf("invalid or unsupported proto %s", string(b))
}

func (p Proto) String() string {
	return string(p)
}

type Method string

const (
	Get    Method = "GET"
	Post   Method = "POST"
	Head   Method = "HEAD"
	Put    Method = "PUT"
	Delete Method = "DELETE"
)

func ParseMethodFromBytes(b []byte) (Method, error) {
	if bytes.Equal([]byte(Get), b) {
		return Get, nil
	}
	if bytes.Equal([]byte(Post), b) {
		return Post, nil
	}
	if bytes.Equal([]byte(Head), b) {
		return Head, nil
	}
	if bytes.Equal([]byte(Put), b) {
		return Put, nil
	}
	if bytes.Equal([]byte(Delete), b) {
		return Delete, nil
	}
	return "", fmt.Errorf("invalid or unsupported method %s", string(b))
}

const CLRF = "\r\n"

func (m Method) String() string {
	return string(m)
}

type Header interface {
	io.WriterTo

	Add(key, value string)
	Set(key, value string)
	Get(key string) string
	Del(key string)
	Has(key string) bool
	Len() int
	Reset()
}

type State int8

const (
	// StateInactive Start state. A client is in StateInactive while it is trying to dial the server's endpoint.
	// If the dial fails, it goes into StateTerminated. If the dial succeeds, it goes into StateActive.
	StateInactive State = iota

	// StateActive Intermediate state. The client has a persistent connection to the server and can issue requests.
	StateActive

	// StateClosedByServer Terminal state. The server has sent a `Connection: close` header in the response. The client
	// cannot send any more requests. The client processes the final response. The underlying transport connection is
	//	// closed by both peers.
	StateClosedByServer

	// StateClosedByClient Terminal state. The client has sent a `Connection: close` header in the request. The client
	// cannot send any more requests. The client processes the final response. The underlying transport connection is
	// closed by both peers.
	StateClosedByClient

	// StateTerminated Terminal state. An error occurred and the underlying transport connection has been closed
	// ungracefully (i.e. without signaling the close with `Connection: Close`.
	StateTerminated
)

// Client makes requests to a server over a persistent connection.
type Client interface {
	Do(*http.Request) (*http.Response, error)
	AsyncDo(*http.Request, func(error, *http.Response))

	State() State

	io.Closer
	Closed() bool

	sonic.Layered[sonic.Conn]
}
