package multicast

import (
	"fmt"
	"net"
	"net/netip"
)

func parseAddr(addr string) (netip.Addr, error) {
	ip, err := netip.ParseAddr(addr)
	if err != nil {
		return netip.Addr{}, err
	}
	if !ip.IsValid() {
		return netip.Addr{}, fmt.Errorf("address=%s not valid", addr)
	}
	return ip, nil
}

func parseMulticastAddr(addr string) (netip.Addr, error) {
	ip, err := parseAddr(addr)
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

func resolveInterfaceIP(iff *net.Interface, ipAddr string) (netip.Addr, error) {
	interfaceAddrs, err := iff.Addrs()
	if err != nil {
		return netip.Addr{}, err
	}

	desiredIP, err := parseAddr(ipAddr)
	if err != nil {
		return netip.Addr{}, err
	}

	var (
		interfaceIP netip.Addr
		found       = false
	)
	for _, interfaceAddr := range interfaceAddrs {
		interfaceIP, err = parseAddr(interfaceAddr.String())
		if err != nil {
			return netip.Addr{}, err
		}
		if interfaceIP == desiredIP {
			found = true
			break
		}
	}

	if !found {
		return netip.Addr{}, fmt.Errorf("desired interface IP=%s is not bound to interface %s", iff.Name, ipAddr)
	} else {
		return interfaceIP, nil
	}
}
