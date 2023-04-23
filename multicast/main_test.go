package multicast

import (
	"fmt"
	"log"
	"net"
	"net/netip"
	"os"
	"testing"
)

var (
	testInterfacesIPv4 []net.Interface
	testInterfacesIPv6 []net.Interface
)

func TestMain(m *testing.M) {
	iffs, err := net.Interfaces()
	if err != nil {
		panic(fmt.Errorf("cannot get interfaces err=%v", err))
	}
	for _, iff := range iffs {
		if iff.Flags&net.FlagMulticast == 0 {
			continue
		}

		log.Println("----------------------------------------------------------")

		log.Printf(
			"interface name=%s index=%d mac=%s up=%v loopback=%v multicast=%v",
			iff.Name,
			iff.Index,
			iff.HardwareAddr,
			iff.Flags&net.FlagUp != 0,
			iff.Flags&net.FlagLoopback != 0,
			iff.Flags&net.FlagMulticast != 0,
		)

		addrs, err := iff.Addrs()
		if err != nil {
			panic(err)
		}
		for _, addr := range addrs {
			var ip string
			switch a := addr.(type) {
			case *net.IPNet:
				ip = a.IP.String()
			case *net.IPAddr:
				ip = a.IP.String()
			default:
				log.Printf("unsupported interface address for name=%s", iff.Name)
				continue
			}

			pa, err := netip.ParseAddr(ip)
			if err != nil {
				panic(err)
			}
			if pa.Is4() || pa.Is4In6() {
				log.Printf(
					"ipv4_addresses:: interface name=%s address=%s ip=%s",
					iff.Name, addr.String(), addr.Network())
				testInterfacesIPv4 = append(testInterfacesIPv4, iff)
			} else if pa.Is6() {
				log.Printf(
					"ipv6_addresses:: interface name=%s address=%s ip=%s",
					iff.Name, addr.String(), addr.Network())
				testInterfacesIPv6 = append(testInterfacesIPv4, iff)
			}
		}
	}

	os.Exit(m.Run())
}
