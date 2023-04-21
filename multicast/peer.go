package multicast

import (
	"fmt"
	"github.com/talostrading/sonic"
	"github.com/talostrading/sonic/internal"
	"github.com/talostrading/sonic/net/ipv4"
	"github.com/talostrading/sonic/sonicerrors"
	"io"
	"net"
	"net/netip"
	"syscall"
)

type UDPPeer struct {
	ioc        *sonic.IO
	socket     *sonic.Socket
	localAddr  *net.UDPAddr
	ipv        int // either 4 or 6
	read       *readReactor
	write      *writeReactor
	outbound   *net.Interface
	outboundIP netip.Addr
	loop       bool

	slot internal.PollData

	sockAddr   syscall.Sockaddr
	closed     bool
	dispatched int
}

func NewUDPPeer(ioc *sonic.IO, network string, addr string) (*UDPPeer, error) {
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
		ioc:       ioc,
		socket:    socket,
		localAddr: localAddr,
		ipv:       ipv,
	}
	p.read = &readReactor{peer: p}
	p.write = &writeReactor{peer: p}

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

func (p *UDPPeer) Read(b []byte) (int, netip.AddrPort, error) {
	return p.socket.RecvFrom(b, 0)
}

func (p *UDPPeer) AsyncRead(b []byte, fn func(error, int, netip.AddrPort)) {
	p.read.b = b
	p.read.fn = fn

	if p.dispatched < sonic.MaxCallbackDispatch {
		p.asyncReadNow(b, func(err error, n int, addr netip.AddrPort) {
			p.dispatched++
			fn(err, n, addr)
			p.dispatched--
		})
	} else {
		p.scheduleRead(fn)
	}
}

func (p *UDPPeer) asyncReadNow(b []byte, fn func(error, int, netip.AddrPort)) {
	n, addr, err := p.Read(b)

	if err == nil {
		fn(err, n, addr)
		return
	}

	if err == sonicerrors.ErrWouldBlock {
		p.scheduleRead(fn)
	} else {
		fn(err, 0, addr)
	}
}

func (p *UDPPeer) scheduleRead(fn func(error, int, netip.AddrPort)) {
	if p.Closed() {
		fn(io.EOF, 0, netip.AddrPort{})
	} else {
		p.slot.Set(internal.ReadEvent, p.read.on)

		if err := p.setRead(); err != nil {
			fn(err, 0, netip.AddrPort{})
		} else {
			p.ioc.RegisterRead(&p.slot)
		}
	}
}

func (p *UDPPeer) setRead() error {
	return p.ioc.SetRead(p.socket.RawFd(), &p.slot)
}

func (p *UDPPeer) Write(b []byte, addr netip.AddrPort) (int, error) {
	return p.socket.SendTo(b, 0, addr)
}

func (p *UDPPeer) AsyncWrite(b []byte, addr netip.AddrPort, fn func(error, int)) {
	p.write.b = b
	p.write.addr = addr
	p.write.fn = fn

	if p.dispatched < sonic.MaxCallbackDispatch {
		p.asyncWriteNow(b, addr, func(err error, n int) {
			p.dispatched++
			fn(err, n)
			p.dispatched--
		})
	} else {
		p.scheduleWrite(fn)
	}
}

func (p *UDPPeer) asyncWriteNow(b []byte, addr netip.AddrPort, fn func(error, int)) {
	n, err := p.Write(b, addr)

	if err == nil {
		fn(err, n)
		return
	}

	if err == sonicerrors.ErrWouldBlock {
		p.scheduleWrite(fn)
	} else {
		fn(err, 0)
	}
}

func (p *UDPPeer) scheduleWrite(fn func(error, int)) {
	if p.Closed() {
		fn(io.EOF, 0)
	} else {
		p.slot.Set(internal.WriteEvent, p.write.on)

		if err := p.setWrite(); err != nil {
			fn(err, 0)
		} else {
			p.ioc.RegisterWrite(&p.slot)
		}
	}
}

func (p *UDPPeer) setWrite() error {
	return p.ioc.SetWrite(p.socket.RawFd(), &p.slot)
}

func (p *UDPPeer) LocalAddr() *net.UDPAddr {
	return p.localAddr
}

func (p *UDPPeer) Close() error {
	if !p.closed {
		p.closed = true
		return p.socket.Close()
	}
	return nil
}

func (p *UDPPeer) Closed() bool {
	return p.closed
}
