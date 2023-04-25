package ipv4

import (
	"github.com/talostrading/sonic"
	"syscall"
	"unsafe"
)

const IP_MULTICAST_ALL = 49 /* grepped torvalds/linux */

func SetMulticastAll(socket *sonic.Socket, all bool) error {
	// BSD makes more sense here. See peer_ipv4_linux_test.go for an explanation.
	var v byte = 0
	if all {
		v = 1
	}
	return syscall.SetsockoptByte(socket.RawFd(), syscall.IPPROTO_IP, IP_MULTICAST_ALL, v)
}
func GetMulticastAll(socket *sonic.Socket) (bool, error) {
	// GetsockoptByte is not defined
	var (
		v byte
		n int
	)
	_, _, errno := syscall.Syscall6(
		uintptr(syscall.SYS_GETSOCKOPT),
		uintptr(socket.RawFd()),
		uintptr(syscall.IPPROTO_IP),
		uintptr(IP_MULTICAST_ALL),
		uintptr(unsafe.Pointer(&v)),
		uintptr(unsafe.Pointer(&n)),
		0,
	)
	var err error
	if errno != 0 {
		err = errno
		return false, err
	}
	if v == 1 {
		return true, nil
	} else {
		return false, nil
	}
}
