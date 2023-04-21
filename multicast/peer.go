package multicast

import (
	"fmt"
	"github.com/talostrading/sonic"
	"github.com/talostrading/sonic/net/ipv4"
	"net"
	"net/netip"
	"syscall"
)

type UDPPeer struct {
	socket     *sonic.Socket
	localAddr  *net.UDPAddr
	ipv        int // either 4 or 6
	outbound   *net.Interface
	outboundIP netip.Addr
	loop       bool

	sockAddr syscall.Sockaddr
}

func NewUDPPeer(network string, addr string) (*UDPPeer, error) {
	resolvedAddr, err := net.ResolveUDPAddr(network, addr)
	if err != nil {
		return nil, fmt.Errorf("could not resolve addr=%s err=%v", addr, err)
	}
	if resolvedAddr.IP == nil {
		if network == "udp" || network == "udp4" {
			resolvedAddr.IP = net.IPv4zero
		} else if network == "udp6" {
			resolvedAddr.IP = net.IPv6zero
		} else {
			return nil, fmt.Errorf("invalid network %s, can only give udp, udp4 or udp6", network)
		}
	}

	domain := sonic.SocketDomainFromIP(resolvedAddr.IP)
	socket, err := sonic.NewSocket(domain, sonic.SocketTypeDatagram, 0)
	if err != nil {
		return nil, fmt.Errorf("could not create socket domain=%s err=%v", domain, err)
	}

	if err := socket.SetNonblocking(true); err != nil {
		return nil, fmt.Errorf("cannot make socket nonblocking")
	}

	// Allow multiple sockets to bind to the same address.
	if err := socket.ReusePort(true); err != nil {
		return nil, fmt.Errorf("cannot make socket reuse the port")
	}

	if err := socket.Bind(resolvedAddr.AddrPort()); err != nil {
		return nil, fmt.Errorf("cannot bind socket to addr=%s err=%v", resolvedAddr, err)
	}

	sockAddr, err := syscall.Getsockname(socket.RawFd())
	if err != nil {
		return nil, fmt.Errorf("cannot get socket address err=%v", err)
	}

	var (
		localAddr = &net.UDPAddr{}
		ipv       int
	)
	switch sa := sockAddr.(type) {
	case *syscall.SockaddrInet4:
		addrPort := netip.AddrPortFrom(netip.AddrFrom4(sa.Addr), uint16(sa.Port))
		localAddr.IP = addrPort.Addr().AsSlice()
		localAddr.Port = int(addrPort.Port())
		ipv = 4
	case *syscall.SockaddrInet6:
		addrPort := netip.AddrPortFrom(netip.AddrFrom16(sa.Addr), uint16(sa.Port))
		localAddr.IP = addrPort.Addr().AsSlice()
		localAddr.Port = int(addrPort.Port())
		localAddr.Zone = addrPort.Addr().Zone()
		ipv = 6
	default:
		return nil, fmt.Errorf("cannot resolve local socket address")
	}

	p := &UDPPeer{
		socket:    socket,
		localAddr: localAddr,
		ipv:       ipv,
	}

	if ipv == 4 {
		p.outboundIP, err = ipv4.GetMulticastInterface(p.socket)
		if err != nil {
			return nil, err
		}

		p.loop, err = ipv4.GetMulticastLoop(p.socket)
		if err != nil {
			return nil, err
		}
	} else {
		// TODO
	}

	return p, nil
}

func (p *UDPPeer) NextLayer() *sonic.Socket {
	return p.socket
}

func (p *UDPPeer) SetOutboundIPv4(interfaceName string) error {
	iff, err := resolveInterface(interfaceName)
	if err != nil {
		return err
	}

	outboundIP, err := ipv4.SetMulticastInterface(p.socket, iff)
	if err != nil {
		return nil
	}

	p.outbound = iff
	p.outboundIP = outboundIP

	return nil
}

func (p *UDPPeer) Outbound() (*net.Interface, netip.Addr) {
	return p.outbound, p.outboundIP
}

func (p *UDPPeer) SetLoop(loop bool) error {
	if err := ipv4.SetMulticastLoop(p.socket, loop); err != nil {
		return err
	} else {
		p.loop = loop
		return nil
	}
}

func (p *UDPPeer) Loop() bool {
	return p.loop
}

func (p *UDPPeer) Join(multicastIP string) error {
	ip, err := parseMulticastIP(multicastIP)
	if err != nil {
		return err
	}

	if ip.Is4() || ip.Is4In6() {
		return p.joinIPv4(ip)
	} else if ip.Is6() {
		return p.joinIPv6(ip)
	} else {
		return fmt.Errorf("unknown IP addressing scheme for addr=%s", multicastIP)
	}
}

func (p *UDPPeer) joinIPv4(ip netip.Addr) error {
	if err := ipv4.AddMembership(p.socket, ip); err != nil {
		return err
	}

	return nil
}

func (p *UDPPeer) joinIPv6(ip netip.Addr) error {
	panic("IPv6 multicast peer not yet supported")
}

func (p *UDPPeer) RecvFrom(b []byte) (int, netip.AddrPort, error) {
	return p.socket.RecvFrom(b, 0)
}

func (p *UDPPeer) SendTo(b []byte, peerAddr netip.AddrPort) (int, error) {
	return p.socket.SendTo(b, 0, peerAddr)
}

func (p *UDPPeer) LocalAddr() *net.UDPAddr {
	return p.localAddr
}

func (p *UDPPeer) Close() error {
	return p.socket.Close()
}
