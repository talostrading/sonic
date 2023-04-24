//go:build darwin || netbsd || freebsd || openbsd || dragonfly

package sonic

import (
	"fmt"
	"net"
	"syscall"
)

// BindToDevice binds the socket to the device with the given name. The device is a network interface (`ip link` to see
// all interfaces).
//
// This makes it such that only packets from the given device will be processed by the socket.
func (s *Socket) BindToDevice(name string) (*net.Interface, error) {
	iff, err := net.InterfaceByName(name)
	if err != nil {
		return nil, err
	}

	if s.domain == SocketDomainIPv4 {
		if err := syscall.SetsockoptInt(
			s.fd,
			syscall.IPPROTO_IP,
			syscall.IP_BOUND_IF,
			iff.Index,
		); err != nil {
			return nil, err
		} else {
			s.boundInterface = iff
			return iff, nil
		}
	} else {
		return nil, fmt.Errorf("cannot yet bind to device when domain is ipv6")
	}
}

// UnbindFromDevice is not working, and honestly I have no clue why.
func (s *Socket) UnbindFromDevice() error {
	if s.boundInterface == nil {
		return nil
	}

	if s.domain == SocketDomainIPv4 {
		_, _, errno := syscall.Syscall6(
			uintptr(syscall.SYS_SETSOCKOPT),
			uintptr(s.fd),
			uintptr(syscall.IPPROTO_IP),
			uintptr(syscall.IP_BOUND_IF),
			0, 0, 0,
		)
		if errno != 0 {
			var err error
			err = errno
			return err
		} else {
			s.boundInterface = nil
			return nil
		}
	} else {
		return fmt.Errorf("cannot yet bind to device when domain is ipv6")
	}
}
