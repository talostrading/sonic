package sonic

import (
	"net"
	"time"

	"github.com/talostrading/sonic/internal"
)

var _ Conn = &conn{}

type conn struct {
	*file
	sock *internal.Socket
}

func createConn(ioc *IO, sock *internal.Socket) *conn {
	c := &conn{
		file: &file{
			ioc: ioc,
			fd:  sock.Fd,
		},
		sock: sock,
	}
	c.pd.Fd = c.fd

	return c
}

func Dial(ioc *IO, network, addr string) (Conn, error) {
	return DialTimeout(ioc, network, addr, 0)
}

func DialTimeout(ioc *IO, network, addr string, timeout time.Duration) (Conn, error) {
	sock, err := internal.NewSocket()
	if err != nil {
		return nil, err
	}

	err = sock.ConnectTimeout(network, addr, timeout)
	if err != nil {
		return nil, err
	}

	return createConn(ioc, sock), nil
}

func (c *conn) LocalAddr() net.Addr {
	return c.sock.LocalAddr
}
func (c *conn) RemoteAddr() net.Addr {
	return c.sock.RemoteAddr
}

func (c *conn) SetDeadline(t time.Time) error {
	panic("not supported")
}
func (c *conn) SetReadDeadline(t time.Time) error {
	panic("not supported")
}
func (c *conn) SetWriteDeadline(t time.Time) error {
	panic("not supported")
}
