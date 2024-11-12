package multicast

import (
	"fmt"
	"io"
	"net"
	"net/netip"
	"syscall"

	"github.com/talostrading/sonic"
	"github.com/talostrading/sonic/internal"
	"github.com/talostrading/sonic/net/ipv4"
	"github.com/talostrading/sonic/sonicerrors"
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
	all        bool

	slot internal.Slot

	sockAddr   syscall.Sockaddr
	closed     bool
	dispatched int
}

// NewUDPPeer creates a new UDPPeer capable of reading/writing multicast packets
// on a network interface.
//
// `network` must be one of: "udp", "udp4", "udp6". "udp" resolves to "udp4". 4
// means IPv4. 6 means IPv6.
//
// `addr` must be `"<ip>:<port>"`. `<ip>` can be one of:
//
//   - "": in this case the peer will be bound to all local interfaces i.e.
//     `INADDR_ANY`. This means the underlying socket will receive datagrams from
//     any groups that have been joined with `Join`, `JoinOn`, `JoinSource` and
//     `JoinSourceOn` and that deliver to `<port>`. This is the most permissive IP
//     address.
//
//   - "<some_multicast_ip>": only traffic originating from
//     <some_multicast_ip>:<port> will be received by this peer. Any other traffic
//     originating from another <multicast_ip>:<port> will be ignored, even if you
//     `Join` that other <multicast_ip>.
//
//   - "<interface_ip_address>": this binds the underlying socket to the IP
//     address owned by one of the host's network interfaces. Reads and writer
//     will only be done from that interface.
//
// A few examples: - If you want to only receive data from 224.0.1.0:1234, then:
// - NewUDPPeer(ioc, "udp", "224:0.1.0:1234") and then Join("224.0.1.0").
// Without the Join, you will get no data.
//
// - If you want to only receive data from a specific interface from
// 224.0.1.0:1234 then: NewUDPPeer(ioc, "udp", "224:0.1.0:1234") and then
// JoinOn("224.0.1.0", "eth0") where "eth0" is the interface name.
//
// - If there are multiple hosts publishing to multicast group 224.0.1.0:1234,
// and you only want to receive data from the host with IP 192.168.1.1 then:
// NewUDPPeer(ioc, "udp", "224:0.1.0:1234"), then JoinSource("224.0.1.0",
// "192.168.1.1").
//
// - If there are multiple hosts publishing to multicast group 224.0.1.0:1234,
// and you want to receive data from all but not the one with IP 192.168.1.1,
// then: NewUDPPeer(ioc, "udp", "224:0.1.0:1234"), then Join("224.0.1.0"),
// BlockSource("192.168.1.1").
//
// The above examples should cover everything you need to write a decent
// multicast app.
//
// Note that multiple UDPPeers can share the same addr. We set ReusePort on the
// socket before binding it.
//
// Note that a peer's port must match the multicast group ports that it wants to
// join. In other words, if an exchange tells you it publishes to
// 224.0.25.108:14326, you must call either NewUDPPeer(ioc, "udp", ":14326") or
// NewUDPPeer(ioc, "udp", "224.0.25.108:14326"). Putting anything other than
// "14326" as a port will make it such that your Peer won't receive any data,
// even though it joined "224.0.25.108".
//
// General note: binding to 0.0.0.0 means telling the TCP/IP layer to use all
// available interfaces for listening and choose the best interface for sending.
//
// Multicast addresses map directly to MAC addresses. Hence, multicast needs
// direct hardware support. A network interface usually only listens to ethernet
// frames destined for its own MAC address. Telling a network interface to join
// a group means telling it to also listen to frames destined to the unique MAC
// mapping of that multicast IP.
//
// Joining a multicast group means that all multicast traffic for all ports on
// that multicast IP will be received by your network interface. However, only
// the traffic destined to your binded listening port will get passed up the
// TCP/IP stack and into your process. Remember, multicast is an IP thing, ports
// is a transport (TCP/UDP) thing.
//
// When sending multicast traffic, you should either bind the peer to 0.0.0.0 or
// to an interface's address. Find those with `ip addr`.
//
// Note that there are certain semantics tied to certain multicast address
// ranges (https://www.rfc-editor.org/rfc/rfc5771). For example: - 224.0.0.0 to
// 224.0.0.255 are not routable, so packets sent to these groups will only reach
// hosts at most 1 link away. So packets won't be forwarded by routers. -
// 224.0.1.0 to 224.0.1.255 are fully routable That means that your choice of
// multicast IP matters, expecially if you're going to write to it. For example,
// let's assume you make both a writer and a reader in the same process and
// SetLoop(true) is set (it is by default): - if the multicast IP is in
// 224.0.0/24 (first case above), the reader will receive the packets once.
// SetLoop(true) ensures the reader gets the packets. - if the multicast IP is
// in 224.0.1/24 (second case above), the reader will receive each packet twice.
// Once from SetLoop(true) and once from the router. Your network router sees
// the packets sent by the writer and destined to a routable multicast IP coming
// in and it routes them back to your machine.
func NewUDPPeer(ioc *sonic.IO, network string, addr string) (*UDPPeer, error) {
	resolvedAddr, err := net.ResolveUDPAddr(network, addr)

	if err != nil {
		return nil, fmt.Errorf("could not resolve addr=%s err=%v", addr, err)
	}
	if resolvedAddr.IP == nil {
		switch network {
		case "udp", "udp4":
			resolvedAddr.IP = net.IPv4zero
		case "udp6":
			resolvedAddr.IP = net.IPv6zero
		default:
			return nil, fmt.Errorf(
				"invalid network %s, can only give udp, udp4 or udp6", network)
		}
	}

	domain := sonic.SocketDomainFromIP(resolvedAddr.IP)
	socket, err := sonic.NewSocket(domain, sonic.SocketTypeDatagram, 0)
	if err != nil {
		return nil, fmt.Errorf(
			"could not create socket domain=%s err=%v", domain, err)
	}

	if err := socket.SetNonblocking(true); err != nil {
		return nil, fmt.Errorf("cannot make socket nonblocking")
	}

	// Allow multiple sockets to bind to the same address.
	if err := socket.ReusePort(true); err != nil {
		return nil, fmt.Errorf("error on REUSE_PORT")
	}

	// Allow address aliasing i.e. can bind to both 0.0.0.0:1234 and
	// 192.168.0.1:1234 on the same device. If ReuseAddr is false, the bind on
	// 192.168.0.1:1234 will fail because we already have somebody bound on port
	// 1234 to all addresses, and 192.168.0.1 is part of those "all" addresses.
	if err := socket.ReuseAddr(true); err != nil {
		return nil, fmt.Errorf("error on socket REUSE_ADDR")
	}

	if err := socket.Bind(resolvedAddr.AddrPort()); err != nil {
		return nil, fmt.Errorf(
			"cannot bind socket to addr=%s err=%v", resolvedAddr, err)
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
		addrPort := netip.AddrPortFrom(
			netip.AddrFrom4(sa.Addr), uint16(sa.Port))
		localAddr.IP = addrPort.Addr().AsSlice()
		localAddr.Port = int(addrPort.Port())
		ipv = 4
	case *syscall.SockaddrInet6:
		addrPort := netip.AddrPortFrom(
			netip.AddrFrom16(sa.Addr), uint16(sa.Port))
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
		all:       false, // this is set in the ipv if-check
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

		if err := ipv4.SetMulticastAll(p.socket, false); err != nil {
			return nil, err
		}
	}

	return p, nil
}

func (p *UDPPeer) NextLayer() *sonic.Socket {
	return p.socket
}

// SetOutboundIPv4 sets the interface on which packets are sent by this peer to
// an IPv4 multicast group.
//
// This means Write(...) and AsyncWrite(...)  will use the specified interface
// to send packets to the multicast group.
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

// SetOutboundIPv6 sets the interface on which packets are sent by this peer to
// an IPv6 multicast group.
//
// This means Write(...) and AsyncWrite(...)  will use the specified interface
// to send packets to the multicast group.
func (p *UDPPeer) SetOutboundIPv6(interfaceName string) error {
	panic("IPv6 not supported")
}

// Outbound returns the interface with which packets are sent to a multicast
// group in calls to Write(...) or AsyncWrite(...).
//
// If SetOutboundIPv4 or SetOuboundIPv6 have not been called before Outbound(),
// the returned interface will be nil and the IP will be 0.0.0.0.
func (p *UDPPeer) Outbound() (*net.Interface, netip.Addr) {
	return p.outbound, p.outboundIP
}

// SetInbound Specify that you only want to receive data from the specified
// interface.
//
// Warning: this might not work on all hosts and interfaces. Instead, you should
// use JoinOn(multicastGroup, interfaceName).
func (p *UDPPeer) SetInbound(interfaceName string) (err error) {
	p.inbound, err = p.socket.BindToDevice(interfaceName)
	return err
}

// Inbound returns the interface set with SetInbound(...). If none was set, it
// returns nil.
func (p *UDPPeer) Inbound() *net.Interface {
	return p.inbound
}

// SetLoop If true, multicast packets sent on one host are looped back to
// readers on the same host. This should be the default.
//
// Use `sysctl net.inet.ip.mcast.loop` to see what is the default on BSD/Linux.
//
// Having this set to true, which is the default, makes it easy to write
// multicast tests on a single host. Anything you write to a multicast group
// will be made available to receivers that joined that multicast group.
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

// SetTTL Sets the time-to-live of udp multicast datagrams. The TTL is 1 by
// default.
//
// A TTL of 1 prevents datagrams from being forwarded beyond the local network.
// Acceptable values are in the range [0, 255]. It is up to the caller to make
// sure the uint8 arg does not overflow.
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

// SetAll is Linux only. If true, all UDPPeer's bound to IP 0.0.0.0 i.e.
// INADDR_ANY will receive all multicast packets from a group that was joined by
// another UDPPeer. Even though this is true in Linux by default, we set it to
// false in NewUDPPeer. Multicast should be opt-in, not opt-out. This flag does
// not exist on BSD systems.
//
// Example: two readers, both on 0.0.0.0:1234 (Note that binding two sockets on
// the same addr+port is allowed since we mark sockets with ReusePort in
// NewUDPPeer). Reader 1 joins 224.0.1.0. Now you would expect only reader 1 to
// get datagrams, but reader 2 gets datagrams as well. This is only on Linux. On
// BSD, only reader 1 gets datagrams.
func (p *UDPPeer) SetAll(all bool) error {
	if err := ipv4.SetMulticastAll(p.socket, all); err != nil {
		return err
	} else {
		p.all = all
		return nil
	}
}

func (p *UDPPeer) All() bool {
	return p.all
}

type IP string
type SourceIP string
type InterfaceName string

// Join a multicast group IP in order to receive data. Since no interface is
// specified, the system uses a default. To receive multicast packets from a
// group on a specific interface, use JoinOn.
//
// The argument multicastIP must be an IP address. It must not have a port in
// it. See the Note above NewUDPPeer for more information.
//
// Joining an already joined IP will return EADDRINUSE.
func (p *UDPPeer) Join(multicastIP IP) error {
	return p.JoinOn(multicastIP, "")
}

// JoinOn a specific interface.
func (p *UDPPeer) JoinOn(multicastIP IP, interfaceName InterfaceName) error {
	return p.JoinSourceOn(multicastIP, "", interfaceName)
}

// JoinSource on any interface. This makes it such that you only receive data
// from the unicast IP sourceIP, even though multiple sourceIPs can send that to
// the same group.
func (p *UDPPeer) JoinSource(multicastIP IP, sourceIP SourceIP) error {
	return p.JoinSourceOn(multicastIP, sourceIP, "")
}

// JoinSourceOn combines JoinSource with JoinOn.
func (p *UDPPeer) JoinSourceOn(
	multicastIP IP,
	sourceIP SourceIP,
	interfaceName InterfaceName,
) error {
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
		return fmt.Errorf(
			"unknown IP addressing scheme for addr=%s", multicastIP)
	}
}

func (p *UDPPeer) joinIPv4(
	multicastIP netip.Addr,
	iff *net.Interface,
	sourceIP netip.Addr,
) (err error) {
	empty := netip.Addr{}
	if sourceIP == empty {
		err = ipv4.AddMembership(p.socket, multicastIP, iff)
	} else {
		err = ipv4.AddSourceMembership(p.socket, multicastIP, sourceIP, iff)
	}
	return
}

func (p *UDPPeer) joinIPv6(
	ip netip.Addr,
	iff *net.Interface,
	sourceIP netip.Addr,
) error {
	panic("IPv6 multicast peer not yet supported")
}

// Leave Leaves the multicast group  Join or JoinOn.
//
// Note that this operation might take some time, potentially minutes, to be
// fulfilled. This is all NIC/switch/router dependent, there is nothing you can
// do about it on the application layer.
func (p *UDPPeer) Leave(multicastIP IP) error {
	return p.LeaveSource(multicastIP, "")
}

// LeaveSource Leaves the multicast group joined with JoinSource or
// JoinSourceOn.
//
// Note that this operation might take some time, potentially minutes, to be
// fulfilled. This is all NIC/switch/router dependent, there is nothing you can
// do about it on the application layer.
func (p *UDPPeer) LeaveSource(multicastIP IP, sourceIP SourceIP) error {
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

	if mip.Is4() || mip.Is4In6() {
		return p.leaveIPv4(mip, sip)
	} else if mip.Is6() {
		return p.leaveIPv6(mip, sip)
	} else {
		return fmt.Errorf(
			"unknown IP addressing scheme for addr=%s", multicastIP)
	}
}

func (p *UDPPeer) leaveIPv4(multicastIP, sourceIP netip.Addr) (err error) {
	empty := netip.Addr{}
	if sourceIP == empty {
		err = ipv4.DropMembership(p.socket, multicastIP)
	} else {
		err = ipv4.DropSourceMembership(p.socket, multicastIP, sourceIP)
	}
	return
}

func (p *UDPPeer) leaveIPv6(multicastIP, sourceIP netip.Addr) error {
	panic("IPv6 multicast peer not yet supported")
}

// BlockSource Makes it such that any data originating from unicast IP sourceIP
// is not going to be received anymore after joining the multicastIP with Join,
// JoinOn, JoinSource or JoinSourceOn.
func (p *UDPPeer) BlockSource(multicastIP IP, sourceIP SourceIP) error {
	mip, err := parseMulticastIP(string(multicastIP))
	if err != nil {
		return err
	}

	sip, err := parseIP(string(sourceIP))
	if err != nil {
		return err
	}

	if mip.Is4() || mip.Is4In6() {
		return p.blockIPv4(mip, sip)
	} else if mip.Is6() {
		return p.blockIPv6(mip, sip)
	} else {
		return fmt.Errorf(
			"unknown IP addressing scheme for addr=%s", multicastIP)
	}
}

func (p *UDPPeer) blockIPv4(multicastIP, sourceIP netip.Addr) (err error) {
	return ipv4.BlockSource(p.socket, multicastIP, sourceIP)
}

func (p *UDPPeer) blockIPv6(multicastIP, sourceIP netip.Addr) (err error) {
	panic("IPv6 multicast peer not yet supported")
}

// UnblockSource undoes BlockSource.
func (p *UDPPeer) UnblockSource(multicastIP IP, sourceIP SourceIP) error {
	mip, err := parseMulticastIP(string(multicastIP))
	if err != nil {
		return err
	}

	sip, err := parseIP(string(sourceIP))
	if err != nil {
		return err
	}

	if mip.Is4() || mip.Is4In6() {
		return p.unblockIPv4(mip, sip)
	} else if mip.Is6() {
		return p.unblockIPv6(mip, sip)
	} else {
		return fmt.Errorf(
			"unknown IP addressing scheme for addr=%s", multicastIP)
	}
}

func (p *UDPPeer) unblockIPv4(multicastIP, sourceIP netip.Addr) (err error) {
	return ipv4.UnblockSource(p.socket, multicastIP, sourceIP)
}

func (p *UDPPeer) unblockIPv6(multicastIP, sourceIP netip.Addr) (err error) {
	panic("IPv6 multicast peer not yet supported")
}

func (p *UDPPeer) Read(b []byte) (int, netip.AddrPort, error) {
	return p.socket.RecvFrom(b, 0)
}

func (p *UDPPeer) SetAsyncReadBuffer(to []byte) {
	p.read.b = to
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

		if err := p.ioc.SetRead(&p.slot); err != nil {
			fn(err, 0, netip.AddrPort{})
		} else {
			p.stats.async.scheduledReads++
			p.ioc.Register(&p.slot)
		}
	}
}

func (p *UDPPeer) Write(b []byte, addr netip.AddrPort) (int, error) {
	return p.socket.SendTo(b, 0, addr)
}

func (p *UDPPeer) AsyncWrite(
	b []byte,
	addr netip.AddrPort,
	fn func(error, int),
) {
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

func (p *UDPPeer) asyncWriteNow(
	b []byte,
	addr netip.AddrPort,
	fn func(error, int),
) {
	n, err := p.Write(b, addr)

	if err == nil {
		fn(err, n)
		return
	}

	if err == sonicerrors.ErrWouldBlock ||
		err == sonicerrors.ErrNoBufferSpaceAvailable {
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

		if err := p.ioc.SetWrite(&p.slot); err != nil {
			fn(err, 0)
		} else {
			p.ioc.Register(&p.slot)
		}
	}
}

// LocalAddr of the peer. Note that the IP can be zero if addr is empty in
// NewUDPPeer.
func (p *UDPPeer) LocalAddr() *net.UDPAddr {
	return p.localAddr
}

func (p *UDPPeer) Close() error {
	if !p.closed {
		p.closed = true
		_ = p.ioc.UnsetReadWrite(&p.slot)
		p.ioc.Deregister(&p.slot)
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

func GetAddressesForInterface(name string) ([]netip.Addr, error) {
	iff, err := net.InterfaceByName(name)
	if err != nil {
		return nil, err
	}

	addrs, err := iff.Addrs()
	if err != nil {
		return nil, err
	}

	var ret []netip.Addr
	for _, addr := range addrs {
		var ipStr string
		switch a := addr.(type) {
		case *net.IPNet:
			ipStr = a.IP.String()
		case *net.IPAddr:
			ipStr = a.IP.String()
		default:
			return nil, fmt.Errorf("address is of unsupported type")
		}
		parsedAddr, err := netip.ParseAddr(ipStr)
		if err != nil {
			return nil, err
		}
		ret = append(ret, parsedAddr)
	}
	return ret, nil
}

func FilterIPv4(addrs []netip.Addr) (ret []netip.Addr) {
	for _, addr := range addrs {
		if addr.Is4() || addr.Is4In6() {
			ret = append(ret, addr)
		}
	}
	return ret
}

func FilterIPv6(addrs []netip.Addr) (ret []netip.Addr) {
	for _, addr := range addrs {
		if addr.Is6() {
			ret = append(ret, addr)
		}
	}
	return ret
}
