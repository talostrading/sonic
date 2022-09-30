package sonic

import (
	"net"
	"time"

	"github.com/talostrading/sonic/internal"
	"github.com/talostrading/sonic/sonicopts"
)

var (
	_ Conn     = &conn{}
	_ net.Conn = &conn{}
)

type conn struct {
	File
	sock *internal.Socket
}

func createConn(ioc *IO, sock *internal.Socket, opts ...sonicopts.Option) *conn {
	nonBlckOpt := sonicopts.Nonblocking(true)
	for _, val := range opts {
		if val.Type() == sonicopts.TypeNonblocking {
			nonBlckOpt = val
		}
	}
	var f File
	if nonBlckOpt.Value().(bool) {
		f = newFile(ioc, sock.Fd)
	} else {
		f = newBlockingFile(ioc, sock.Fd)
	}

	c := &conn{
		File: f,
		sock: sock,
	}
	return c
}

func Dial(ioc *IO, network, addr string, opts ...sonicopts.Option) (Conn, error) {
	return DialTimeout(ioc, network, addr, 0, opts...)
}

func DialTimeout(ioc *IO, network, addr string, timeout time.Duration, opts ...sonicopts.Option) (Conn, error) {
	sock, err := internal.NewSocket(opts...)
	if err != nil {
		return nil, err
	}

	err = sock.ConnectTimeout(network, addr, timeout)
	if err != nil {
		return nil, err
	}
	return createConn(ioc, sock, opts...), nil
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
