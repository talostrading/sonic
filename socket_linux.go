//go:build linux

package sonic

import (
	"net"
	"syscall"
	"unsafe"
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

func (s *Socket) UnbindFromDevice() error {
	if s.boundInterface == nil {
		return nil
	}

	_, _, errno := syscall.Syscall6(
		uintptr(syscall.SYS_SETSOCKOPT),
		uintptr(s.fd),
		uintptr(syscall.SOL_SOCKET),
		uintptr(syscall.SO_BINDTODEVICE),
		uintptr(unsafe.Pointer(&[]byte("_")[0])),
		0, 0,
	)
	if errno != 0 {
		var err error
		err = errno
		return err
	} else {
		s.boundInterface = nil
		return nil
	}
	//if err := syscall.SetsockoptString(
	//	s.fd,
	//	syscall.SOL_SOCKET,
	//	syscall.SO_BINDTODEVICE,
	//	"",
	//); err != nil {
	//} else {
	//	s.boundInterface = nil
	//	return nil
	//}
}
