package sonic

import "net"

var _ PacketConn = &MulticastClient{}

type MulticastClient struct {
	*packetConn

	ioc *IO
	iff *net.Interface
}

func NewMulticastClient(ioc *IO, iff *net.Interface) (*MulticastClient, error) {
	c := &MulticastClient{
		ioc: ioc,
		iff: iff,
	}
	c.packetConn = &packetConn{
		ioc: ioc,
	}
	return c, nil
}

func (c *MulticastClient) Join(multicastAddr net.Addr, sourceAddrs ...net.Addr) {

}

func (c *MulticastClient) Leave(multicastAddr net.Addr, sourceAddrs ...net.Addr) {

}

func (c *MulticastClient) Block(sourceAddr net.Addr) {

}

func (c *MulticastClient) Unblock(sourceAddr net.Addr) {

}

func (c *MulticastClient) Close() error {
	//c.Leave()
	return c.packetConn.Close()
}
