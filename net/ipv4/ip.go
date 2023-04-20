package ipv4

import (
	"fmt"
	"github.com/talostrading/sonic"
	"net/netip"
	"syscall"
	"unsafe"
)

var (
	req     = &syscall.IPMreq{}
	inAddr4 [4]byte
	size    int
)

func reset() {
	for i := 0; i < len(req.Multiaddr); i++ {
		req.Multiaddr[i] = 0
	}
	for i := 0; i < len(req.Interface); i++ {
		req.Interface[i] = 0
	}
	for i := 0; i < len(inAddr4); i++ {
		inAddr4[i] = 0
	}
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

func SetMulticastInterface(socket *sonic.Socket, ip netip.Addr) error {
	if err := checkIP(ip); err != nil {
		return err
	}

	reset()

	copy(inAddr4[:], ip.AsSlice())

	_, _, errno := syscall.Syscall6(
		syscall.SYS_GETSOCKOPT,
		uintptr(socket.RawFd()),
		uintptr(syscall.IPPROTO_IP),
		uintptr(syscall.IP_MULTICAST_IF),
		uintptr(unsafe.Pointer(&inAddr4)),
		4,
		0,
	)
	var err error
	if errno != 0 {
		err = errno
		return err
	}
	return nil
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
