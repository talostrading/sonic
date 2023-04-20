package multicast

import (
	"fmt"
	"github.com/talostrading/sonic"
	"net"
	"net/netip"
	"syscall"
)

type Addr string

var (
	AddrAny = Addr(net.IPv4zero.String())
)

type UDPPeer struct {
	socket    *sonic.Socket
	localAddr *net.UDPAddr
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

	if err := socket.Bind(resolvedAddr.AddrPort()); err != nil {
		return nil, fmt.Errorf("cannot bind socket to addr=%s err=%v", resolvedAddr, err)
	}

	if err := socket.SetNonblocking(true); err != nil {
		return nil, fmt.Errorf("cannot make socket nonblocking")
	}

	// Allow multiple sockets to listen to the same multicast group.
	if err := socket.ReusePort(true); err != nil {
		return nil, fmt.Errorf("cannot make socket reuse the port")
	}

	sockAddr, err := syscall.Getsockname(socket.RawFd())
	if err != nil {
		return nil, fmt.Errorf("cannot get socket address err=%v", err)
	}

	localAddr := &net.UDPAddr{}
	switch sa := sockAddr.(type) {
	case *syscall.SockaddrInet4:
		addrPort := netip.AddrPortFrom(netip.AddrFrom4(sa.Addr), uint16(sa.Port))
		localAddr.IP = addrPort.Addr().AsSlice()
		localAddr.Port = int(addrPort.Port())
	case *syscall.SockaddrInet6:
		addrPort := netip.AddrPortFrom(netip.AddrFrom16(sa.Addr), uint16(sa.Port))
		localAddr.IP = addrPort.Addr().AsSlice()
		localAddr.Port = int(addrPort.Port())
		localAddr.Zone = addrPort.Addr().Zone()
	default:
		return nil, fmt.Errorf("cannot resolve local socket address")
	}

	p := &UDPPeer{
		socket:    socket,
		localAddr: localAddr,
	}

	return p, nil
}

func (p *UDPPeer) LocalAddr() *net.UDPAddr {
	return p.localAddr
}

func (p *UDPPeer) Close() error {
	return p.socket.Close()
}
