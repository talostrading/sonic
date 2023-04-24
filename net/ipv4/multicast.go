package ipv4

import (
	"fmt"
	"github.com/talostrading/sonic"
	"net"
	"net/netip"
	"syscall"
)

func GetMulticastInterfaceAddr(socket *sonic.Socket) (netip.Addr, error) {
	addr, err := syscall.GetsockoptInet4Addr(socket.RawFd(), syscall.IPPROTO_IP, syscall.IP_MULTICAST_IF)
	if err != nil {
		return netip.Addr{}, err
	} else {
		return netip.AddrFrom4(addr), nil
	}
}

func GetMulticastInterfaceAddrAndGroup(socket *sonic.Socket) (interfaceAddr, multicastAddr netip.Addr, err error) {
	addr, err := syscall.GetsockoptIPMreq(socket.RawFd(), syscall.IPPROTO_IP, syscall.IP_MULTICAST_IF)
	if err != nil {
		return netip.Addr{}, netip.Addr{}, err
	} else {
		return netip.AddrFrom4(addr.Interface), netip.AddrFrom4(addr.Multiaddr), nil
	}
}

func GetMulticastInterfaceIndex(socket *sonic.Socket) (int, error) {
	return syscall.GetsockoptInt(socket.RawFd(), syscall.IPPROTO_IP, syscall.IP_MULTICAST_IF)
}

func SetMulticastInterface(socket *sonic.Socket, iff *net.Interface) (netip.Addr, error) {
	if iff.Flags&net.FlagMulticast == 0 {
		return netip.Addr{}, fmt.Errorf("interface=%s does not support multicast", iff.Name)
	}

	addrs, err := iff.Addrs()
	if err != nil {
		return netip.Addr{}, err
	}

	var (
		interfaceAddr net.IP
		found         = false
	)
	for _, addr := range addrs {
		switch a := addr.(type) {
		case *net.IPAddr:
			interfaceAddr = a.IP
		case *net.IPNet:
			interfaceAddr = a.IP
		}
		if interfaceAddr.To4() != nil {
			found = true
			break
		}
	}

	if found {
		var addr [4]byte
		copy(addr[:], interfaceAddr)

		if err := syscall.SetsockoptInet4Addr(
			socket.RawFd(),
			syscall.IPPROTO_IP,
			syscall.IP_MULTICAST_IF,
			addr,
		); err != nil {
			return netip.Addr{}, err
		}
		return netip.AddrFrom4(addr), nil
	} else {
		return netip.Addr{}, fmt.Errorf("interface has no IPv4 address assigned")
	}
}

func SetMulticastLoop(socket *sonic.Socket, loop bool) error {
	v := 0
	if loop {
		v = 1
	}
	return syscall.SetsockoptInt(
		socket.RawFd(),
		syscall.IPPROTO_IP,
		syscall.IP_MULTICAST_LOOP,
		v,
	)
}

func GetMulticastLoop(socket *sonic.Socket) (bool, error) {
	v, err := syscall.GetsockoptInt(
		socket.RawFd(),
		syscall.IPPROTO_IP,
		syscall.IP_MULTICAST_LOOP,
	)
	if err != nil {
		return false, err
	} else if v == 0 {
		return true, nil
	} else {
		return false, nil
	}
}

func SetMulticastTTL(socket *sonic.Socket, ttl uint8) error {
	return syscall.SetsockoptInt(
		socket.RawFd(),
		syscall.IPPROTO_IP,
		syscall.IP_MULTICAST_TTL,
		int(ttl),
	)
}

func GetMulticastTTL(socket *sonic.Socket) (uint8, error) {
	ttl, err := syscall.GetsockoptInt(
		socket.RawFd(),
		syscall.IPPROTO_IP,
		syscall.IP_MULTICAST_TTL,
	)
	return uint8(ttl), err
}

func ValidateMulticastIP(ip netip.Addr) error {
	if !ip.Is4() && !ip.Is4In6() {
		return fmt.Errorf("expected an IPv4 address=%s", ip)
	}
	if !ip.IsMulticast() {
		return fmt.Errorf("expected a multicast address=%s", ip)
	}
	return nil
}

// AddMembership makes the given socket a member of the specified multicast IP.
func AddMembership(
	socket *sonic.Socket,
	multicastIP netip.Addr,
	iff *net.Interface,
) error {
	mreq := &syscall.IPMreq{}
	copy(mreq.Multiaddr[:], multicastIP.AsSlice())

	if iff != nil {
		addrs, err := iff.Addrs()
		if err != nil {
			return err
		}

		set := false
		for _, addr := range addrs {
			var (
				ip    net.IP
				parse = false
			)

			switch a := addr.(type) {
			case *net.IPAddr:
				ip = a.IP
				parse = true
			case *net.IPNet:
				ip = a.IP
				parse = true
			case *net.UDPAddr:
				ip = a.IP
				parse = true
			}

			if parse {
				parsedIP, err := netip.ParseAddr(ip.String())
				if err != nil {
					return err
				}
				if parsedIP.Is4() || parsedIP.Is4In6() {
					copy(mreq.Interface[:], parsedIP.AsSlice())
					set = true
					break
				}
			}
		}
		if !set {
			return fmt.Errorf("cannot add membership on interface %s as there is no IPv4 address on it", iff.Name)
		}
	}

	return syscall.SetsockoptIPMreq(socket.RawFd(), syscall.IPPROTO_IP, syscall.IP_ADD_MEMBERSHIP, mreq)
}
