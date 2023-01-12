package sonic

import (
	"net"
	"time"
)

var _ PacketConn = &packetConn{}

type packetConn struct {
	ioc       *IO
	localAddr net.Addr
	closed    uint32
}

func newPacketConn(ioc *IO, localAddr net.Addr) *packetConn {
	return &packetConn{ioc: ioc, localAddr: localAddr}
}

func (c *packetConn) AsyncReadFrom(b []byte, cb func(err error, n int, addr net.Addr)) {

}

func (c *packetConn) ReadFrom(p []byte) (n int, addr net.Addr, err error) {
	//TODO implement me
	panic("implement me")
}

func (c *packetConn) WriteTo(p []byte, addr net.Addr) (n int, err error) {
	//TODO implement me
	panic("implement me")
}

func (c *packetConn) AsyncWriteTo(b []byte, cb func(err error, n int, addr net.Addr)) {

}

func (c *packetConn) Close() error {
	//TODO implement me
	panic("implement me")
}

func (c *packetConn) LocalAddr() net.Addr {
	return c.localAddr
}

func (c *packetConn) SetDeadline(t time.Time) error {
	//TODO implement me
	panic("implement me")
}

func (c *packetConn) SetReadDeadline(t time.Time) error {
	//TODO implement me
	panic("implement me")
}

func (c *packetConn) SetWriteDeadline(t time.Time) error {
	//TODO implement me
	panic("implement me")
}
