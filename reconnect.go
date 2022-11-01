package sonic

import (
	"github.com/talostrading/sonic/sonicopts"
	"net"
	"time"
)

var _ ReconnectingConn = &reconnectingConn{}

type reconnectingConn struct {
	ioc     *IO
	network string
	addr    string
	opts    []sonicopts.Option
}

func NewReconnectingConn(ioc *IO, network, addr string, opts ...sonicopts.Option) (ReconnectingConn, error) {
	conn := &reconnectingConn{
		ioc:     ioc,
		network: network,
		addr:    addr,
		opts:    opts,
	}
	return conn, nil
}

func (c *reconnectingConn) Reconnect() error {
	//TODO implement me
	panic("implement me")
}

func (c *reconnectingConn) Read(p []byte) (n int, err error) {
	//TODO implement me
	panic("implement me")
}

func (c *reconnectingConn) Write(p []byte) (n int, err error) {
	//TODO implement me
	panic("implement me")
}

func (c *reconnectingConn) AsyncRead(b []byte, cb AsyncCallback) {
	//TODO implement me
	panic("implement me")
}

func (c *reconnectingConn) AsyncReadAll(b []byte, cb AsyncCallback) {
	//TODO implement me
	panic("implement me")
}

func (c *reconnectingConn) CancelReads() {
	//TODO implement me
	panic("implement me")
}

func (c *reconnectingConn) AsyncWrite(b []byte, cb AsyncCallback) {
	//TODO implement me
	panic("implement me")
}

func (c *reconnectingConn) AsyncWriteAll(b []byte, cb AsyncCallback) {
	//TODO implement me
	panic("implement me")
}

func (c *reconnectingConn) CancelWrites() {
	//TODO implement me
	panic("implement me")
}

func (c *reconnectingConn) Close() error {
	//TODO implement me
	panic("implement me")
}

func (c *reconnectingConn) Closed() bool {
	//TODO implement me
	panic("implement me")
}

func (c *reconnectingConn) RawFd() int { // TODO this should be obtained through multiple NextLayer calls.
	//TODO implement me
	panic("implement me")
}

func (c *reconnectingConn) Opts() []sonicopts.Option {
	//TODO implement me
	panic("implement me")
}

func (c *reconnectingConn) LocalAddr() net.Addr {
	//TODO implement me
	panic("implement me")
}

func (c *reconnectingConn) RemoteAddr() net.Addr {
	//TODO implement me
	panic("implement me")
}

func (c *reconnectingConn) SetDeadline(t time.Time) error {
	//TODO implement me
	panic("implement me")
}

func (c *reconnectingConn) SetReadDeadline(t time.Time) error {
	//TODO implement me
	panic("implement me")
}

func (c *reconnectingConn) SetWriteDeadline(t time.Time) error {
	//TODO implement me
	panic("implement me")
}

func (c reconnectingConn) NextLayer() Conn {
	//TODO implement me
	panic("implement me")
}
