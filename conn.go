package sonic

import (
	"fmt"
	"net"
	"time"

	"github.com/talostrading/sonic/internal"
)

var _ Conn = &conn{}

type conn struct {
	*file
	fd int

	localAddr  net.Addr
	remoteAddr net.Addr
}

func Dial(ioc *IO, network, addr string) (Conn, error) {
	if network[:3] != "tcp" {
		return nil, fmt.Errorf("network must start with tcp for Dial")
	}
	return DialTimeout(ioc, network, addr, 10*time.Second)
}

func DialTimeout(ioc *IO, network, addr string, timeout time.Duration) (Conn, error) {
	fd, localAddr, remoteAddr, err := internal.ConnectTimeout(network, addr, timeout)
	if err != nil {
		return nil, err
	}

	return newConn(ioc, fd, localAddr, remoteAddr), nil
}

func newConn(ioc *IO, fd int, localAddr, remoteAddr net.Addr) *conn {
	return &conn{
		file:       &file{ioc: ioc, fd: fd, pd: internal.PollData{Fd: fd}},
		fd:         fd,
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
	return fmt.Errorf("not supported")
}
func (c *conn) SetReadDeadline(t time.Time) error {
	return fmt.Errorf("not supported")
}
func (c *conn) SetWriteDeadline(t time.Time) error {
	return fmt.Errorf("not supported")
}
