//go:build linux

package sonic

import (
	"net"
	"syscall"
	"unsafe"
)

// BindToDevice binds the socket to the device with the given name. The device
// must be a network interface (`ip link` to see all interfaces).
//
// This makes it such that only packets from the given device will be processed
// by the socket.
func (s *Socket) BindToDevice(name string) (*net.Interface, error) {
	iff, err := net.InterfaceByName(name)
	if err != nil {
		return nil, err
	}

	if err := syscall.SetsockoptString(
		s.fd,
		syscall.SOL_SOCKET,
		syscall.SO_BINDTODEVICE,
		iff.Name,
	); err != nil {
		return nil, err
	} else {
		s.boundInterface = iff
		return iff, nil
	}
}

// UnbindFromDevice is not working, and honestly I have no clue why.
func (s *Socket) UnbindFromDevice() error {
	if s.boundInterface == nil {
		return nil
	}

	/* #nosec G103 -- the use of unsafe has been audited */
	_, _, errno := syscall.Syscall6(
		uintptr(syscall.SYS_SETSOCKOPT),
		uintptr(s.fd),
		uintptr(syscall.SOL_SOCKET),
		uintptr(syscall.SO_BINDTODEVICE),
		uintptr(unsafe.Pointer(&[]byte("_")[0])),
		0, 0,
	)
	if errno != 0 {
		err := errno
		return err
	} else {
		s.boundInterface = nil
		return nil
	}
}

func GetBoundDevice(fd int) (string, error) {
	into := make([]byte, syscall.IFNAMSIZ)
	n := 0

	/* #nosec G103 -- the use of unsafe has been audited */
	_, _, errno := syscall.Syscall6(
		uintptr(syscall.SYS_GETSOCKOPT),
		uintptr(fd),
		uintptr(syscall.SOL_SOCKET),
		uintptr(syscall.SO_BINDTODEVICE),
		uintptr(unsafe.Pointer(&(into[0]))),
		uintptr(unsafe.Pointer(&n)),
		0,
	)
	if errno != 0 {
		err := errno
		return "", err
	} else {
		return string(into[:n]), nil
	}
}
