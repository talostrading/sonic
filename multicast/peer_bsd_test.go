//go:build darwin || netbsd || freebsd || openbsd || dragonfly

package multicast

import (
	"fmt"
	"github.com/talostrading/sonic"
	"testing"
)

func TestUDPPeer_SetOutboundInterfaceOnUnspecifiedIPandPort(t *testing.T) {
	ioc := sonic.MustIO()
	defer ioc.Close()

	peer, err := NewUDPPeer(ioc, "udp", "")
	if err != nil {
		t.Fatal(err)
	}
	defer peer.Close()

	if err := peer.SetOutboundIPv4("en0"); err != nil {
		t.Fatal(err)
	}

	outboundInterface, outboundIP := peer.Outbound()
	fmt.Println("outbound", outboundInterface, outboundIP)
}

func TestUDPPeer_SetOutboundInterfaceOnUnspecifiedPort(t *testing.T) {
	ioc := sonic.MustIO()
	defer ioc.Close()

	peer, err := NewUDPPeer(ioc, "udp", "localhost:0")
	if err != nil {
		t.Fatal(err)
	}
	defer peer.Close()

	if err := peer.SetOutboundIPv4("en0"); err != nil {
		t.Fatal(err)
	}

	outboundInterface, outboundIP := peer.Outbound()
	fmt.Println("outbound", outboundInterface, outboundIP)
}
