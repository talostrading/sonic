package sonicwebsocket

import (
	"crypto/tls"

	"github.com/talostrading/sonic"
)

type Client struct {
}

func (c *Client) Read(b []byte) (int, error) {
	panic("implement me")
}

func (c *Client) Write(b []byte) (int, error) {
	panic("implement me")
}

func (c *Client) AsyncDial(addr string, cb sonic.AsyncCallback) {

}

func (c *Client) AsyncDialTLS(addr string, cnf *tls.Config, cb func(error, *Client)) {

}

func (c *Client) AsyncRead(b []byte, cb sonic.AsyncCallback) {

}

func (c *Client) AsyncReadAll(b []byte, cb sonic.AsyncCallback) {

}

func (c *Client) AsyncWrite(b []byte, cb sonic.AsyncCallback) {

}

func (c *Client) AsyncWriteAll(b []byte, cb sonic.AsyncCallback) {

}
