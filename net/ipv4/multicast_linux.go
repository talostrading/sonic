//go:build linux

package ipv4

import (
	"github.com/talostrading/sonic"
	"syscall"
)

func SetMulticastAll(sock *sonic.Socket, all bool) error {
	v := 0
	if all {
		v = 1
	}
	return syscall.SetsockoptInt(sock.RawFd(), syscall.IPPROTO_IP, syscall.IP_MULTICAST_ALL, v)
}
