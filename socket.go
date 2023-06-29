package sonic

import (
	"fmt"
	"io"
	"net"
	"net/netip"
	"syscall"

	"github.com/talostrading/sonic/sonicerrors"
	"golang.org/x/sys/unix"
)

type SocketDomain int

const (
	SocketDomainUnix SocketDomain = iota
	SocketDomainIPv4
	SocketDomainIPv6
)

func (s SocketDomain) into() (int, error) {
	switch s {
	case SocketDomainUnix:
		return syscall.AF_UNIX, nil
	case SocketDomainIPv4:
		return syscall.AF_INET, nil
	case SocketDomainIPv6:
		return syscall.AF_INET6, nil
	}
	return -1, fmt.Errorf("(socket domain not supported)")
}

func SocketDomainFromIP(ip net.IP) SocketDomain {
	if ip.To4() != nil {
		return SocketDomainIPv4
	} else {
		return SocketDomainIPv6
	}
}

func (s SocketDomain) String() string {
	switch s {
	case SocketDomainUnix:
		return "unix"
	case SocketDomainIPv4:
		return "ipv4"
	case SocketDomainIPv6:
		return "ipv6"
	}
	return "(unknown domain)"
}

type SocketType int

const (
	SocketTypeStream SocketType = iota
	SocketTypeDatagram
	SocketRaw
)

func (s SocketType) into() (int, error) {
	switch s {
	case SocketTypeStream:
		return syscall.SOCK_STREAM, nil
	case SocketTypeDatagram:
		return syscall.SOCK_DGRAM, nil
	case SocketRaw:
		return syscall.SOCK_RAW, nil
	}
	return -1, fmt.Errorf("(socket type not supported)")
}

func (s SocketType) String() string {
	switch s {
	case SocketTypeStream:
		return "stream"
	case SocketTypeDatagram:
		return "datagram"
	case SocketRaw:
		return "raw"
	}
	return "(unknown type)"
}

type SocketProtocol int

const (
	SocketProtocolTCP SocketProtocol = iota
	SocketProtocolUDP
)

func (s SocketProtocol) into() (int, error) {
	return 0, nil
}

func (s SocketProtocol) String() string {
	switch s {
	case SocketProtocolTCP:
		return "tcp"
	case SocketProtocolUDP:
		return "udp"
	}
	return "(unknown type)"
}

type Socket struct {
	domain            SocketDomain
	socketType        SocketType
	protocol          SocketProtocol
	readSockAddr      syscall.Sockaddr
	writeSockAddrIpv4 *syscall.SockaddrInet4
	fd                int
	boundInterface    *net.Interface
}

func NewSocket(
	domain SocketDomain,
	socketType SocketType,
	protocol SocketProtocol, // ignored for now
) (*Socket, error) {
	rawDomain, err := domain.into()
	if err != nil {
		return nil, err
	}
	rawType, err := socketType.into()
	if err != nil {
		return nil, err
	}

	s := &Socket{
		domain:            domain,
		socketType:        socketType,
		protocol:          protocol,
		writeSockAddrIpv4: &syscall.SockaddrInet4{},
		fd:                -1,
	}

	s.fd, err = syscall.Socket(rawDomain, rawType, 0)
	if err != nil {
		return nil, err
	}

	return s, nil
}

func (s *Socket) SetNonblocking(nonblocking bool) error {
	return syscall.SetNonblock(s.fd, nonblocking)
}

func (s *Socket) IsNonblocking() (bool, error) {
	flags, err := unix.FcntlInt(uintptr(s.fd), unix.F_GETFL, 0)
	if err != nil {
		return false, err
	}

	return flags&unix.O_NONBLOCK != 0, nil
}

func (s *Socket) Bind(addrPort netip.AddrPort) error {
	var sa syscall.Sockaddr
	if addrPort.Addr().Is4() || addrPort.Addr().Is4In6() {
		sa = &syscall.SockaddrInet4{
			Port: int(addrPort.Port()),
			Addr: addrPort.Addr().As4(),
		}
	} else if addrPort.Addr().Is6() {
		sa = &syscall.SockaddrInet6{
			Port: int(addrPort.Port()),
			Addr: addrPort.Addr().As16(),
			// TODO the zoneId is only meant for link-local addressing so by definition, to populate this
			// you need an interface. The interface index is then the zoneId.
			ZoneId: 0,
		}
	} else {
		return fmt.Errorf("cannot bind socket to addr=%s", addrPort)
	}
	return syscall.Bind(s.fd, sa)
}

func (s *Socket) ReuseAddr(reuse bool) error {
	v := 0
	if reuse {
		v = 1
	}

	return syscall.SetsockoptInt(s.fd, syscall.SOL_SOCKET, unix.SO_REUSEADDR, v)
}

func (s *Socket) ReusePort(reuse bool) error {
	v := 0
	if reuse {
		v = 1
	}

	return syscall.SetsockoptInt(s.fd, syscall.SOL_SOCKET, unix.SO_REUSEPORT, v)
}

func (s *Socket) SetNoDelay(delay bool) error {
	if s.protocol != SocketProtocolTCP {
		return fmt.Errorf("NoDelay is a TCP protocol specific socket option")
	}

	v := 0
	if delay {
		v = 1
	}

	return syscall.SetsockoptInt(s.fd, syscall.IPPROTO_TCP, syscall.TCP_NODELAY, v)
}

type SocketIOFlags int

func (s *Socket) RecvFrom(
	b []byte,
	flags SocketIOFlags, /* not yet usable */
) (n int, peerAddr netip.AddrPort, err error) {
	n, s.readSockAddr, err = syscall.Recvfrom(s.fd, b, 0)
	if err != nil {
		if err == syscall.EWOULDBLOCK || err == syscall.EAGAIN {
			return 0, netip.AddrPort{}, sonicerrors.ErrWouldBlock
		}
		return 0, netip.AddrPort{}, err
	}
	if n == 0 {
		return 0, netip.AddrPort{}, io.EOF
	}

	if n < 0 {
		n = 0
	}

	switch sa := s.readSockAddr.(type) {
	case *syscall.SockaddrInet4:
		return n, netip.AddrPortFrom(netip.AddrFrom4(sa.Addr), uint16(sa.Port)), err
	case *syscall.SockaddrInet6:
		return n, netip.AddrPortFrom(netip.AddrFrom16(sa.Addr), uint16(sa.Port)), err
	default:
		return n, netip.AddrPort{}, fmt.Errorf("can only recvfrom ipv4 and ipv6 peers err=%v", err)
	}
}

func (s *Socket) SendTo(
	b []byte,
	flags SocketIOFlags, /* not yet usable */
	peerAddr netip.AddrPort,
) (int, error) {
	s.writeSockAddrIpv4.Addr = peerAddr.Addr().As4()
	s.writeSockAddrIpv4.Port = int(peerAddr.Port())
	if err := syscall.Sendto(s.fd, b, 0, s.writeSockAddrIpv4); err == nil {
		return len(b), nil
	} else if err == syscall.EWOULDBLOCK || err == syscall.EAGAIN {
		return 0, sonicerrors.ErrWouldBlock
	} else if err == syscall.ENOBUFS {
		return 0, sonicerrors.ErrNoBufferSpaceAvailable
	} else {
		return 0, err
	}
}

func (s *Socket) Close() (err error) {
	if s.fd >= 0 {
		err = syscall.Close(s.fd)
		s.fd = -1
	}
	return err
}

func (s *Socket) BoundDevice() *net.Interface {
	return s.boundInterface
}

func (s *Socket) RawFd() int {
	return s.fd
}
