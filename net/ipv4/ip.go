package ipv4

import (
	"fmt"
	"github.com/talostrading/sonic"
	"net"
	"net/netip"
	"syscall"
	"unsafe"
)

type IPv4Mreqn struct {
	Multiaddr      [4]byte /* in_addr */
	Interface      [4]byte /* in_addr */
	InterfaceIndex int
}

var (
	req     = &syscall.IPMreq{}
	reqn    = &IPv4Mreqn{}
	inAddr4 [4]byte
	size    int

	SizeOfMreqn = syscall.SizeofIPMreq + unsafe.Sizeof(size /* needs to be an int */)
)

func reset() {
	for i := 0; i < 4; i++ {
		req.Multiaddr[i] = 0
		req.Interface[i] = 0
		reqn.Multiaddr[i] = 0
		reqn.Interface[i] = 0
		inAddr4[i] = 0
	}
	reqn.InterfaceIndex = 0
	size = 0
}

func checkIP(ip netip.Addr) error {
	if !ip.Is4() && !ip.Is4In6() {
		return fmt.Errorf("expected an IPv4 address=%s", ip)
	}
	return nil
}

func GetMulticastInterface(socket *sonic.Socket) (netip.Addr, error) {
	reset()

	_, _, errno := syscall.Syscall6(
		syscall.SYS_GETSOCKOPT,
		uintptr(socket.RawFd()),
		uintptr(syscall.IPPROTO_IP),
		uintptr(syscall.IP_MULTICAST_IF),
		uintptr(unsafe.Pointer(&inAddr4)),
		uintptr(unsafe.Pointer(&size)),
		0,
	)
	var err error
	if errno != 0 {
		err = errno
		return netip.Addr{}, err
	} else {
		return netip.AddrFrom4(inAddr4), nil
	}
}

func SetMulticastInterface(socket *sonic.Socket, iff *net.Interface) (netip.Addr, error) {
	if iff.Flags&net.FlagMulticast == 0 {
		return netip.Addr{}, fmt.Errorf("interface=%s does not support multicast", iff.Name)
	}

	reset()

	addrs, err := iff.Addrs()
	if err != nil {
		return netip.Addr{}, err
	}

	var localAddr net.IP
	for _, addr := range addrs {
		switch a := addr.(type) {
		case *net.IPAddr:
			localAddr = a.IP
		case *net.IPNet:
			localAddr = a.IP
		}
		if localAddr.To4() != nil {
			break
		}
	}

	copy(inAddr4[:], localAddr)

	_, _, errno := syscall.Syscall6(
		syscall.SYS_SETSOCKOPT,
		uintptr(socket.RawFd()),
		uintptr(syscall.IPPROTO_IP),
		uintptr(syscall.IP_MULTICAST_IF),
		uintptr(unsafe.Pointer(&inAddr4)),
		uintptr(4),
		0,
	)

	if errno != 0 {
		err = errno
		return netip.Addr{}, err
	}

	interfaceLocalAddr, _ := netip.AddrFromSlice(localAddr)
	return interfaceLocalAddr, nil
}

// AddMembership makes the given socket a member of the specified multicast IP.
func AddMembership(socket *sonic.Socket, ip netip.Addr) error {
	if !ip.Is4() && !ip.Is4In6() {
		return fmt.Errorf("expected an IPv4 address=%s", ip)
	}

	if !ip.IsMulticast() {
		return fmt.Errorf("expected a multicast address=%s", ip)
	}

	copy(req.Multiaddr[:], ip.AsSlice())

	_, _, errno := syscall.Syscall6(
		syscall.SYS_SETSOCKOPT,
		uintptr(socket.RawFd()),
		uintptr(syscall.IPPROTO_IP),
		uintptr(syscall.IP_ADD_MEMBERSHIP),
		uintptr(unsafe.Pointer(req)),
		syscall.SizeofIPMreq,
		0)

	var err error
	if errno != 0 {
		err = errno
	}

	return err
}
