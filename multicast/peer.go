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

var emptyIPv4Addr = [4]byte{0x0, 0x0, 0x0, 0x0}

type UDPPeer struct {
	ioc        *sonic.IO
	socket     *sonic.Socket
	localAddr  *net.UDPAddr
	ipv        int // either 4 or 6
	stats      *Stats
	read       *readReactor
	write      *writeReactor
	outbound   *net.Interface
	outboundIP netip.Addr
	inbound    *net.Interface
	loop       bool
	ttl        uint8

	slot internal.PollData

	sockAddr   syscall.Sockaddr
	closed     bool
	dispatched int
}

// NewUDPPeer TODO if you pass "" as addr then what, the behaviour changes depending whether you also want to read/write.
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
		return nil, fmt.Errorf("error on REUSE_PORT")
	}
	//
	//// Allow address aliasing i.e. can bind to both 0.0.0.0 and 192.168.0.1 on the same device.
	//if err := socket.ReuseAddr(true); err != nil {
	//	return nil, fmt.Errorf("error on socket REUSE_ADDR")
	//}

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
		stats:     &Stats{},
		ttl:       1,
	}
	p.read = &readReactor{peer: p}
	p.write = &writeReactor{peer: p}
	p.slot.Fd = p.socket.RawFd()

	if ipv == 4 {
		p.outboundIP, err = ipv4.GetMulticastInterfaceAddr(p.socket)
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
	iff, err := resolveMulticastInterface(interfaceName)
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

func (p *UDPPeer) SetOutboundIPv6(interfaceName string) error {
	panic("IPv6 not supported")
}

func (p *UDPPeer) Outbound() (*net.Interface, netip.Addr) {
	return p.outbound, p.outboundIP
}

// SetInbound Specify that you only want to receive data from the specified interface.
//
// Warning: this might not work. Instead, you should use JoinOn(multicastGroup, interfaceName).
func (p *UDPPeer) SetInbound(interfaceName string) (err error) {
	p.inbound, err = p.socket.BindToDevice(interfaceName)
	return err
}

func (p *UDPPeer) Inbound() *net.Interface {
	return p.inbound
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

func (p *UDPPeer) SetTTL(ttl uint8) error {
	if err := ipv4.SetMulticastTTL(p.socket, ttl); err != nil {
		return err
	} else {
		p.ttl = ttl
		return nil
	}
}

func (p *UDPPeer) TTL() uint8 {
	return p.ttl
}

type MulticastIP string
type SourceIP string
type InterfaceName string

// Join a multicast group IP in order to receive data. Since no interface is specified, the system uses a default.
// To receive multicast packets from a group on a specific interface, use JoinOn.
//
// Joining an already joined IP will return EADDRINUSE.
func (p *UDPPeer) Join(multicastIP MulticastIP) error {
	return p.JoinOn(multicastIP, "")
}

func (p *UDPPeer) JoinOn(multicastIP MulticastIP, interfaceName InterfaceName) error {
	return p.JoinSourceOn(multicastIP, "", interfaceName)
}

func (p *UDPPeer) JoinSource(multicastIP MulticastIP, sourceIP SourceIP) error {
	return p.JoinSourceOn(multicastIP, sourceIP, "")
}

func (p *UDPPeer) JoinSourceOn(multicastIP MulticastIP, sourceIP SourceIP, interfaceName InterfaceName) error {
	mip, err := parseMulticastIP(string(multicastIP))
	if err != nil {
		return err
	}

	sip := netip.Addr{}
	if len(string(sourceIP)) > 0 {
		sip, err = parseIP(string(sourceIP))
		if err != nil {
			return err
		}
	}

	var iff *net.Interface
	if string(interfaceName) != "" {
		iff, err = resolveMulticastInterface(string(interfaceName))
		if err != nil {
			return err
		}
	}

	if mip.Is4() || mip.Is4In6() {
		return p.joinIPv4(mip, iff, sip)
	} else if mip.Is6() {
		return p.joinIPv6(mip, iff, sip)
	} else {
		return fmt.Errorf("unknown IP addressing scheme for addr=%s", multicastIP)
	}
}

func (p *UDPPeer) joinIPv4(multicastIP netip.Addr, iff *net.Interface, sourceIP netip.Addr) (err error) {
	empty := netip.Addr{}
	if sourceIP == empty {
		err = ipv4.AddMembership(p.socket, multicastIP, iff)
	} else {
		err = ipv4.AddSourceMembership(p.socket, multicastIP, sourceIP, iff)
	}
	return
}

func (p *UDPPeer) joinIPv6(ip netip.Addr, iff *net.Interface, sourceIP netip.Addr) error {
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
		p.stats.async.immediateReads++
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
			p.stats.async.scheduledReads++
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

	if err == sonicerrors.ErrWouldBlock || err == sonicerrors.ErrNoBufferSpaceAvailable {
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

func (p *UDPPeer) Stats() *Stats {
	return p.stats
}
