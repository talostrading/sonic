//go:build linux

package sonic

import (
	"net"
	"syscall"
)

func (s *Socket) BindToDevice(name string) error {
	iff, err := net.InterfaceByName(name)
	if err != nil {
		return err
	}

	if err := syscall.SetsockoptString(
		s.fd,
		syscall.SOL_SOCKET,
		syscall.SO_BINDTODEVICE,
		iff.Name,
	); err != nil {
		return err
	} else {
		s.boundInterface = iff
		return nil
	}
}
