//go:build linux

package ipv4

func AddMembershipOnInterface(socket *sonic.Socket, multicastIP netip.Addr, interfaceIndex int) error {
	if err := ValidateMulticastIP(multicastIP); err != nil {
		return err
	}

	mreq := &syscall.IPMreqn{}
	copy(mreq.Multiaddr[:], multicastIP.AsSlice())
	mreq.Ifindex = int32(interfaceIndex)

	return syscall.SetsockoptIPMreqn(socket.RawFd(), syscall.IPPROTO_IP, syscall.IP_ADD_MEMBERSHIP, mreq)
}

func DropMembership(socket *sonic.Socket, multicastIP netip.Addr) error {
	if err := ValidateMulticastIP(multicastIP); err != nil {
		return err
	}

	mreq := &syscall.IPMreqn{}
	copy(mreq.Multiaddr[:], multicastIP.AsSlice())

	return syscall.SetsockoptIPMreqn(socket.RawFd(), syscall.IPPROTO_IP, syscall.IP_DROP_MEMBERSHIP, mreq)
}
