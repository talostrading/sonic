package ipv4

import (
	"fmt"
	"github.com/talostrading/sonic"
	"net"
	"net/netip"
	"syscall"
	"unsafe"
)

// SizeofIPMreqSource I would love to do unsafe.SizeOf  but for a struct with 3 4-byte arrays, it returns 8 on my Mac.
// It should return 12 :). So we add 4 bytes instead which is enough for the source IP.
const SizeofIPMreqSource = syscall.SizeofIPMreq + 4

// IPMreqSource adds Sourceaddr to net.IPMreq
type IPMreqSource struct {
	Multiaddr  [4]byte /* in_addr */
	Interface  [4]byte /* in_addr */
	Sourceaddr [4]byte /* in_addr */
}

// TODO this is defined on Linux but not on BSD even though it should so PR in golang to introduce this

// TODO i think everything here should just take the fd as first arg.

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

func prepareAddMembership(multicastIP netip.Addr, iff *net.Interface) (*syscall.IPMreq, error) {
	mreq := &syscall.IPMreq{}
	copy(mreq.Multiaddr[:], multicastIP.AsSlice())

	if iff != nil {
		addrs, err := iff.Addrs()
		if err != nil {
			return nil, err
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
					return nil, err
				}
				if parsedIP.Is4() || parsedIP.Is4In6() {
					copy(mreq.Interface[:], parsedIP.AsSlice())
					set = true
					break
				}
			}
		}
		if !set {
			return nil, fmt.Errorf("cannot add membership on interface %s as there is no IPv4 address on it", iff.Name)
		}
	}

	return mreq, nil
}

// AddMembership makes the given socket a member of the specified multicast IP.
func AddMembership(
	socket *sonic.Socket,
	multicastIP netip.Addr,
	iff *net.Interface,
) error {
	mreq, err := prepareAddMembership(multicastIP, iff)
	if err != nil {
		return err
	}
	return syscall.SetsockoptIPMreq(socket.RawFd(), syscall.IPPROTO_IP, syscall.IP_ADD_MEMBERSHIP, mreq)
}

func AddSourceMembership(
	socket *sonic.Socket,
	multicastIP netip.Addr,
	sourceIP netip.Addr,
	iff *net.Interface,
) error {
	mreq, err := prepareAddMembership(multicastIP, iff)
	if err != nil {
		return err
	}

	mreqSource := &IPMreqSource{}
	copy(mreqSource.Multiaddr[:], mreq.Multiaddr[:])
	copy(mreqSource.Interface[:], mreq.Interface[:])
	copy(mreqSource.Sourceaddr[:], sourceIP.AsSlice())

	_, _, errno := syscall.Syscall6(
		uintptr(syscall.SYS_SETSOCKOPT),
		uintptr(socket.RawFd()),
		uintptr(syscall.IPPROTO_IP),
		uintptr(syscall.IP_ADD_SOURCE_MEMBERSHIP),
		uintptr(unsafe.Pointer(mreqSource)),
		uintptr(SizeofIPMreqSource),
		0)
	if errno != 0 {
		err = errno
	}
	return err
}

func prepareDropMembership(multicastIP netip.Addr) *syscall.IPMreq {
	mreq := &syscall.IPMreq{}
	copy(mreq.Multiaddr[:], multicastIP.AsSlice())

	return mreq
}

func DropMembership(socket *sonic.Socket, multicastIP netip.Addr) error {
	mreq := prepareDropMembership(multicastIP)

	return syscall.SetsockoptIPMreq(socket.RawFd(), syscall.IPPROTO_IP, syscall.IP_DROP_MEMBERSHIP, mreq)
}

func DropSourceMembership(socket *sonic.Socket, multicastIP, sourceIP netip.Addr) (err error) {
	mreq := prepareDropMembership(multicastIP)
	mreqSource := &IPMreqSource{}
	copy(mreqSource.Multiaddr[:], mreq.Multiaddr[:])
	copy(mreqSource.Sourceaddr[:], sourceIP.AsSlice())

	_, _, errno := syscall.Syscall6(
		uintptr(syscall.SYS_SETSOCKOPT),
		uintptr(socket.RawFd()),
		uintptr(syscall.IPPROTO_IP),
		uintptr(syscall.IP_DROP_SOURCE_MEMBERSHIP),
		uintptr(unsafe.Pointer(mreqSource)),
		uintptr(SizeofIPMreqSource),
		0,
	)
	if errno != 0 {
		err = errno
	}
	return err
}

func BlockSource(socket *sonic.Socket, multicastIP, sourceIP netip.Addr) (err error) {
	mreqSource := &IPMreqSource{}
	copy(mreqSource.Multiaddr[:], multicastIP.AsSlice())
	copy(mreqSource.Sourceaddr[:], sourceIP.AsSlice())

	_, _, errno := syscall.Syscall6(
		uintptr(syscall.SYS_SETSOCKOPT),
		uintptr(socket.RawFd()),
		uintptr(syscall.IPPROTO_IP),
		uintptr(syscall.IP_BLOCK_SOURCE),
		uintptr(unsafe.Pointer(mreqSource)),
		uintptr(SizeofIPMreqSource),
		0,
	)
	if errno != 0 {
		err = errno
	}
	return err
}

func UnblockSource(socket *sonic.Socket, multicastIP, sourceIP netip.Addr) (err error) {
	mreqSource := &IPMreqSource{}
	copy(mreqSource.Multiaddr[:], multicastIP.AsSlice())
	copy(mreqSource.Sourceaddr[:], sourceIP.AsSlice())

	_, _, errno := syscall.Syscall6(
		uintptr(syscall.SYS_SETSOCKOPT),
		uintptr(socket.RawFd()),
		uintptr(syscall.IPPROTO_IP),
		uintptr(syscall.IP_UNBLOCK_SOURCE),
		uintptr(unsafe.Pointer(mreqSource)),
		uintptr(SizeofIPMreqSource),
		0,
	)
	if errno != 0 {
		err = errno
	}
	return err
}
