package sonic

import (
	"net"
	"time"

	"github.com/talostrading/sonic/sonicopts"

	"github.com/talostrading/sonic/internal"
)

var _ Conn = &conn{}

type conn struct {
	*file
	localAddr  net.Addr
	remoteAddr net.Addr
}

// Dial establishes a stream based connection to the specified address. It is similar to `net.Dial`.
//
// Data can be sent or received only from the specified address for all networks: tcp, udp and unix domain sockets.
func Dial(
	ioc *IO,
	network, addr string,
	opts ...sonicopts.Option,
) (Conn, error) {
	return DialTimeout(ioc, network, addr, 10*time.Second, opts...)
}

// `DialTimeout` is like `Dial` but with a timeout.
func DialTimeout(
	ioc *IO, network, addr string,
	timeout time.Duration,
	opts ...sonicopts.Option,
) (Conn, error) {
	fd, localAddr, remoteAddr, err := internal.ConnectTimeout(network, addr, timeout, opts...)
	if err != nil {
		return nil, err
	}

	return newConn(ioc, fd, localAddr, remoteAddr), nil
}

func newConn(
	ioc *IO,
	fd int,
	localAddr, remoteAddr net.Addr,
) *conn {
	return &conn{
		file:       newFile(ioc, fd),
		localAddr:  localAddr,
		remoteAddr: remoteAddr,
	}
}

func (c *conn) LocalAddr() net.Addr {
	return c.localAddr
}
func (c *conn) RemoteAddr() net.Addr {
	return c.remoteAddr
}

func (c *conn) SetDeadline(t time.Time) error {
	panic("not implemented")
}
func (c *conn) SetReadDeadline(t time.Time) error {
	panic("not implemented")
}
func (c *conn) SetWriteDeadline(t time.Time) error {
	panic("not implemented")
}

func (c *conn) RawFd() int {
	return c.file.slot.Fd
}
