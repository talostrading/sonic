//go:build darwin || netbsd || freebsd || openbsd || dragonfly

package sonic

import (
	"fmt"
	"net"
	"syscall"
)

func (s *Socket) BindToDevice(name string) error {
	iff, err := net.InterfaceByName(name)
	if err != nil {
		return err
	}

	if s.domain == SocketDomainIPv4 {
		if err := syscall.SetsockoptInt(
			s.fd,
			syscall.IPPROTO_IP,
			syscall.IP_BOUND_IF,
			iff.Index,
		); err != nil {
			return err
		} else {
			s.boundInterface = iff
			return nil
		}
	} else {
		return fmt.Errorf("cannot yet bind to device when domain is ipv6")
	}
}
