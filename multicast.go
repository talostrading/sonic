package sonic

import (
	"fmt"
	"net"
	"syscall"
)

// MulticastClient wraps a PacketConn allowing it to receive data from multiple UDP Multicast groups.
type MulticastClient struct {
	PacketConn

	ioc *IO
	iff *net.Interface

	multicastAddrs []net.Addr
}

func NewMulticastClient(ioc *IO, iff *net.Interface) (*MulticastClient, error) {
	c := &MulticastClient{
		ioc: ioc,
		iff: iff,
	}
	return c, nil
}

// Join joins the multicast group specified by multicastAddr optionally filtering datagrams by the source addresses in
// sourceAddrs.
//
// A MulticastClient may join multiple groups as long as they are all bound to the same port and the client is not
// bound to a specific group address (i.e. it is instead bound to 0.0.0.0).
func (c *MulticastClient) Join(network, addr string, sourceAddrs ...net.Addr) error {
	multicastAddr, err := net.ResolveUDPAddr(network, addr)
	if err != nil {
		return err
	}

	if c.PacketConn != nil {
		return fmt.Errorf("joining multiple groups is not currently supported")
	}

	if !multicastAddr.IP.IsMulticast() {
		return fmt.Errorf("cannot join on non-multicast address %s", multicastAddr)
	}

	if ip := multicastAddr.IP.To4(); ip != nil {
		// IPv4 is not compatible with IPv6 which means devices cannot communicate with each other if they mix
		// addressing. So we want an IPv4 interface address for an IPv4 multicast address and same for IPv6.

		// set multicast address
		mreq := &syscall.IPMreq{}
		copy(mreq.Multiaddr[:], ip)

		// set interface address
		addrs, err := c.iff.Addrs()
		if err != nil {
			return err
		}
		set := false
	outer:
		for _, addr := range addrs {
			switch addr := addr.(type) {
			case *net.IPAddr:
				if ipv4 := addr.IP.To4(); ipv4 != nil {
					copy(mreq.Interface[:], addr.IP)
					set = true
					break outer
				}
			case *net.IPNet:
				if ipv4 := addr.IP.To4(); ipv4 != nil {
					copy(mreq.Interface[:], addr.IP)
					set = true
					break outer
				}
			default:
				return fmt.Errorf("unknown address type")
			}
		}
		if !set {
			return fmt.Errorf("the interface %s does not have an associated IPv4 address", c.iff.Name)
		}

		c.PacketConn, err = NewPacketConn(c.ioc, "udp4", fmt.Sprintf("0.0.0.0:%d", multicastAddr.Port))
		if err != nil {
			return err
		}

		// TODO sourceAddrs with syscall.Syscall6
		err = syscall.SetsockoptIPMreq(c.RawFd(), syscall.IPPROTO_IP, syscall.IP_ADD_MEMBERSHIP, mreq)
		if err != nil {
			c.PacketConn.Close()
		}
		return err
	} else {
		ip = multicastAddr.IP.To16()

		// set multicast address
		mreq := &syscall.IPv6Mreq{}
		copy(mreq.Multiaddr[:], ip)

		// set interface address
		mreq.Interface = uint32(c.iff.Index)

		var err error
		c.PacketConn, err = NewPacketConn(c.ioc, "udp6", multicastAddr.String())
		if err != nil {
			return err
		}

		// TODO sourceAddrs with syscall.Syscall6
		err = syscall.SetsockoptIPv6Mreq(c.RawFd(), syscall.IPPROTO_IPV6, syscall.IPV6_JOIN_GROUP, mreq)
		if err != nil {
			c.PacketConn.Close()
		}
		return err
	}
}

func (c *MulticastClient) Leave(multicastAddr net.Addr, sourceAddrs ...net.Addr) {

}

func (c *MulticastClient) Block(sourceAddr net.Addr) {

}

func (c *MulticastClient) Unblock(sourceAddr net.Addr) {

}

func (c *MulticastClient) Close() error {
	//c.Leave()
	return c.PacketConn.Close()
}

func (c *MulticastClient) MulticastAddrs() []net.Addr {
	return c.multicastAddrs
}
