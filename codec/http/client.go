package http

import (
	"crypto/tls"
	"github.com/talostrading/sonic"
	"net/http"
	"net/url"
)

var _ Client = &client{}

type client struct {
	ioc *sonic.IO
	url *url.URL
	tls *tls.Config

	src *sonic.ByteBuffer
	dst *sonic.ByteBuffer

	state State
	conn  sonic.Conn // TODO this should be a reconnecting conn
}

func NewClient(ioc *sonic.IO, url *url.URL, tls *tls.Config) (*client, error) {
	c := &client{
		ioc: ioc,
		url: url,
		tls: tls,

		src: sonic.NewByteBuffer(),
		dst: sonic.NewByteBuffer(),

		state: StateInactive,
	}
	return c, nil
}

func (c *client) Do(req *http.Request) (*http.Response, error) {
	panic("implement me")
}

func (c *client) AsyncDo(req *http.Request, f func(error, *http.Response)) {
	panic("implement me")
}

func (c *client) State() State {
	return c.state
}

func (c *client) Close() error {
	// TODO http closing handshake
	return c.conn.Close()
}

func (c *client) Closed() bool {
	return c.conn.Closed()
}

func (c *client) NextLayer() sonic.Conn {
	return c.conn
}
