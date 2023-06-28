package sonic

import (
	"fmt"
	"net"
	"syscall"
	"unsafe"
)

func serializeIPv4Addr(addr net.Addr, into []byte) bool {
	copyIPv4 := func(ip net.IP) bool {
		if ipv4 := ip.To4(); ipv4 != nil {
			n := copy(into, ipv4)
			if n >= len(ipv4) {
				return true
			}
		}
		return false
	}

	switch addr := addr.(type) {
	case *net.IPAddr:
		return copyIPv4(addr.IP)
	case *net.IPNet:
		return copyIPv4(addr.IP)
	case *net.UDPAddr:
		return copyIPv4(addr.IP)
	}
	return false
}

func createIPv4InterfaceRequest(
	ip net.IP,
	iff *net.Interface,
) (*syscall.IPMreq, error) {
	// TODO here you might also want to explicitly request
	// a specific IP, but not sure yet about the semantics of the link layer.

	// set multicast address
	mreq := &syscall.IPMreq{}
	copy(mreq.Multiaddr[:], ip)

	// set interface address
	addrs, err := iff.Addrs()
	if err != nil {
		return nil, err
	}

	for _, addr := range addrs {
		if serializeIPv4Addr(addr, mreq.Interface[:]) {
			return mreq, nil
		}
	}

	return nil, fmt.Errorf(
		"interface name=%s index=%d has not IPv4 address addrs=%v",
		iff.Name, iff.Index, addrs)
}

func createIPv4InterfaceRequestWithSource(
	multicastIP net.IP,
	iff *net.Interface,
	sourceIP net.IP,
) (*IPMreqSource, error) {
	if sourceIP.To4() == nil {
		return nil, fmt.Errorf(
			"invalid source=%s; can only filter on IPv4 sources",
			sourceIP,
		)
	}

	mreqAll, err := createIPv4InterfaceRequest(multicastIP, iff)
	if err != nil {
		return nil, err
	}

	mreq := &IPMreqSource{}
	copy(mreq.Interface[:], mreqAll.Interface[:])
	copy(mreq.Multiaddr[:], mreqAll.Multiaddr[:])
	copy(mreq.Sourceaddr[:], sourceIP)

	return mreq, nil
}

func createIPv6InterfaceRequest(
	ip net.IP,
	iff *net.Interface,
) (*syscall.IPv6Mreq, error) {
	// set multicast address
	mreq := &syscall.IPv6Mreq{}
	copy(mreq.Multiaddr[:], ip)

	// set interface address
	mreq.Interface = uint32(iff.Index)

	return mreq, nil
}

func makeInterfaceRequest(
	req MulticastRequestType,
	iff *net.Interface,
	fd int,
	multicastIP, sourceIP net.IP,
) error {
	var (
		errno syscall.Errno
		err   error
	)

	// IPv4 is not compatible with IPv6 which means devices cannot communicate
	// with each other if they mix addressing. So we want an IPv4 interface
	// address for an IPv4 multicast address and same for IPv6.
	if multicastIP := multicastIP.To4(); multicastIP != nil {
		// IPv4
		if sourceIP == nil {
			mreq, err := createIPv4InterfaceRequest(multicastIP, iff)
			if err == nil {
				/* #nosec G103 -- the use of unsafe has been audited */
				_, _, errno = syscall.Syscall6(
					syscall.SYS_SETSOCKOPT,
					uintptr(fd),
					uintptr(syscall.IPPROTO_IP),
					uintptr(req.ToIPv4()),
					uintptr(unsafe.Pointer(mreq)),
					syscall.SizeofIPMreq, 0)
			}
		} else {
			mreq, err := createIPv4InterfaceRequestWithSource(
				multicastIP,
				iff,
				sourceIP,
			)
			if err == nil {
				/* #nosec G103 -- the use of unsafe has been audited */
				_, _, errno = syscall.Syscall6(
					syscall.SYS_SETSOCKOPT,
					uintptr(fd),
					uintptr(syscall.IPPROTO_IP),
					uintptr(req.ToIPv4()),
					uintptr(unsafe.Pointer(mreq)),
					SizeofIPMreqSource, 0)
			}
		}
	} else {
		// IPv6
		if sourceIP == nil {
			mreq, err := createIPv6InterfaceRequest(multicastIP.To16(), iff)
			if err == nil {
				/* #nosec G103 -- the use of unsafe has been audited */
				_, _, errno = syscall.Syscall6(
					syscall.SYS_SETSOCKOPT,
					uintptr(fd),
					uintptr(syscall.IPPROTO_IPV6),
					uintptr(req.ToIPv6()),
					uintptr(unsafe.Pointer(mreq)),
					syscall.SizeofIPv6Mreq, 0,
				)
			}
		} else {
			panic("TODO")
		}
	}

	if errno != 0 {
		err = errno
	}
	return err
}
