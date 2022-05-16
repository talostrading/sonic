package sonic

import (
	"net"
	"time"

	"github.com/talostrading/sonic/internal"
)

var (
	_ Conn     = &conn{}
	_ net.Conn = &conn{}
)

type conn struct {
	*file
	sock *internal.Socket
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

	c := &conn{
		file: &file{
			ioc: ioc,
			fd:  sock.Fd,
		},
		sock: sock,
	}

	return c, nil
}

func (c *conn) LocalAddr() net.Addr {
	return c.sock.LocalAddr
}
func (c *conn) RemoteAddr() net.Addr {
	return c.sock.RemoteAddr
}

func (c *conn) SetDeadline(t time.Time) error {
	// TODO
	return nil
}
func (c *conn) SetReadDeadline(t time.Time) error {
	// TODO
	return nil
}
func (c *conn) SetWriteDeadline(t time.Time) error {
	// TODO
	return nil
}
