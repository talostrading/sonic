package ipv4

import (
	"github.com/talostrading/sonic"
	"syscall"
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
