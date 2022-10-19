package sonic

import (
	"github.com/talostrading/sonic/sonicopts"
	"net"
	"time"

	"github.com/talostrading/sonic/internal"
)

var _ Conn = &conn{}

type conn struct {
	FileDescriptor

	sock *internal.Socket
}

func createConn(ioc *IO, sock *internal.Socket, opts ...sonicopts.Option) (Conn, error) {
	c := &conn{
		sock: sock,
	}

	var err error
	c.FileDescriptor, err = NewFileDescriptor(ioc, sock.Fd, opts...)
	if err != nil {
		return nil, err
	}

	return c, nil
}

func Dial(ioc *IO, network, addr string, opts ...sonicopts.Option) (Conn, error) {
	return DialTimeout(ioc, network, addr, 0, opts...)
}

func DialTimeout(ioc *IO, network, addr string, timeout time.Duration, opts ...sonicopts.Option) (Conn, error) {
	sock, err := internal.NewSocket()
	if err != nil {
		return nil, err
	}

	err = sock.ConnectTimeout(network, addr, timeout)
	if err != nil {
		return nil, err
	}

	return createConn(ioc, sock, opts...)
}

func (c *conn) LocalAddr() net.Addr {
	return c.sock.LocalAddr
}
func (c *conn) RemoteAddr() net.Addr {
	return c.sock.RemoteAddr
}

func (c *conn) SetDeadline(t time.Time) error {
	// TODO
	panic("not supported")
}
func (c *conn) SetReadDeadline(t time.Time) error {
	// TODO
	panic("not supported")
}
func (c *conn) SetWriteDeadline(t time.Time) error {
	// TODO
	panic("not supported")
}
