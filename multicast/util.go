package multicast

import (
	"fmt"
	"net"
	"net/netip"
)

func parseIP(addr string) (netip.Addr, error) {
	ip, err := netip.ParseAddr(addr)
	if err != nil {
		return netip.Addr{}, err
	}
	if !ip.IsValid() {
		return netip.Addr{}, fmt.Errorf("address=%s not valid", addr)
	}
	return ip, nil
}

func parseMulticastIP(addr string) (netip.Addr, error) {
	ip, err := parseIP(addr)
	if err != nil {
		return netip.Addr{}, err
	}

	if !ip.IsMulticast() {
		return netip.Addr{}, fmt.Errorf("addr=%s not multicast", addr)
	}

	return ip, err
}

func resolveInterface(name string) (*net.Interface, error) {
	iff, err := net.InterfaceByName(name)
	if err != nil {
		return nil, err
	}

	if iff.Flags&net.FlagUp == 0 {
		return nil, fmt.Errorf("interface=%s is not up", name)
	}

	if iff.Flags&net.FlagMulticast == 0 {
		return nil, fmt.Errorf("interface=%s does not support multicast", name)
	}

	return iff, nil
}
