package multicast

import (
	"github.com/talostrading/sonic/net/ipv4"
	"net"
	"testing"
)

// Listing multicast group memberships: netstat -gsv

func TestUDPPeer_IPv4Addresses(t *testing.T) {
	{
		peer, err := NewUDPPeer("udp", net.IPv4zero.String())
		if err == nil {
			t.Fatal("should have received an error as the address is missing the port")
		}

		if iface, _ := peer.Outbound(); iface != nil {
			t.Fatal("not explicit outbound interface should have been set")
		}
	}
	{
		peer, err := NewUDPPeer("udp4", net.IPv4zero.String())
		if err == nil {
			t.Fatal("should have received an error as the address is missing the port")
		}

		if iface, _ := peer.Outbound(); iface != nil {
			t.Fatal("not explicit outbound interface should have been set")
		}
	}
	{
		peer, err := NewUDPPeer("udp", "")
		if err != nil {
			t.Fatal(err)
		}
		defer peer.Close()

		addr := peer.LocalAddr()
		if given, expected := addr.IP.String(), net.IPv4zero.String(); given != expected {
			t.Fatalf("given=%s expected=%s", given, expected)
		}
		if addr.Port == 0 {
			t.Fatal("port should not be 0")
		}

		if iface, _ := peer.Outbound(); iface != nil {
			t.Fatal("not explicit outbound interface should have been set")
		}
	}
	{
		peer, err := NewUDPPeer("udp4", "")
		if err != nil {
			t.Fatal(err)
		}
		defer peer.Close()

		addr := peer.LocalAddr()
		if given, expected := addr.IP.String(), net.IPv4zero.String(); given != expected {
			t.Fatalf("given=%s expected=%s", given, expected)
		}
		if addr.Port == 0 {
			t.Fatal("port should not be 0")
		}

		if iface, _ := peer.Outbound(); iface != nil {
			t.Fatal("not explicit outbound interface should have been set")
		}
	}
	{
		peer, err := NewUDPPeer("udp", ":0")
		if err != nil {
			t.Fatal(err)
		}
		defer peer.Close()

		addr := peer.LocalAddr()
		if given, expected := addr.IP.String(), net.IPv4zero.String(); given != expected {
			t.Fatalf("given=%s expected=%s", given, expected)
		}
		if addr.Port == 0 {
			t.Fatal("port should not be 0")
		}

		if iface, _ := peer.Outbound(); iface != nil {
			t.Fatal("not explicit outbound interface should have been set")
		}
	}
	{
		peer, err := NewUDPPeer("udp4", ":0")
		if err != nil {
			t.Fatal(err)
		}
		defer peer.Close()

		addr := peer.LocalAddr()
		if given, expected := addr.IP.String(), net.IPv4zero.String(); given != expected {
			t.Fatalf("given=%s expected=%s", given, expected)
		}
		if addr.Port == 0 {
			t.Fatal("port should not be 0")
		}

		if iface, _ := peer.Outbound(); iface != nil {
			t.Fatal("not explicit outbound interface should have been set")
		}
	}
	{
		peer, err := NewUDPPeer("udp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		defer peer.Close()

		addr := peer.LocalAddr()
		if given, expected := addr.IP.String(), "127.0.0.1"; given != expected {
			t.Fatalf("given=%s expected=%s", given, expected)
		}
		if addr.Port == 0 {
			t.Fatal("port should not be 0")
		}

		if iface, _ := peer.Outbound(); iface != nil {
			t.Fatal("not explicit outbound interface should have been set")
		}
	}
	{
		peer, err := NewUDPPeer("udp4", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		defer peer.Close()

		addr := peer.LocalAddr()
		if given, expected := addr.IP.String(), "127.0.0.1"; given != expected {
			t.Fatalf("given=%s expected=%s", given, expected)
		}
		if addr.Port == 0 {
			t.Fatal("port should not be 0")
		}

		if iface, _ := peer.Outbound(); iface != nil {
			t.Fatal("not explicit outbound interface should have been set")
		}
	}
	{
		peer, err := NewUDPPeer("udp", "localhost:0")
		if err != nil {
			t.Fatal(err)
		}
		defer peer.Close()

		addr := peer.LocalAddr()
		if given, expected := addr.IP.String(), "127.0.0.1"; given != expected {
			t.Fatalf("given=%s expected=%s", given, expected)
		}
		if addr.Port == 0 {
			t.Fatal("port should not be 0")
		}

		if iface, _ := peer.Outbound(); iface != nil {
			t.Fatal("not explicit outbound interface should have been set")
		}
	}
	{
		peer, err := NewUDPPeer("udp4", "localhost:0")
		if err != nil {
			t.Fatal(err)
		}
		defer peer.Close()

		addr := peer.LocalAddr()
		if given, expected := addr.IP.String(), "127.0.0.1"; given != expected {
			t.Fatalf("given=%s expected=%s", given, expected)
		}
		if addr.Port == 0 {
			t.Fatal("port should not be 0")
		}

		if iface, _ := peer.Outbound(); iface != nil {
			t.Fatal("not explicit outbound interface should have been set")
		}
	}
}

func TestUDPPeer_IPv6Addresses(t *testing.T) {
	{
		_, err := NewUDPPeer("udp6", net.IPv6zero.String())
		if err == nil {
			t.Fatal("should have received an error as the address is missing the port")
		}
	}
	{
		peer, err := NewUDPPeer("udp6", "")
		if err != nil {
			t.Fatal(err)
		}
		defer peer.Close()

		addr := peer.LocalAddr()
		if given, expected := addr.IP.String(), net.IPv6zero.String(); given != expected {
			t.Fatalf("given=%s expected=%s", given, expected)
		}
		if addr.Port == 0 {
			t.Fatal("port should not be 0")
		}
	}
	{
		peer, err := NewUDPPeer("udp6", ":0")
		if err != nil {
			t.Fatal(err)
		}
		defer peer.Close()

		addr := peer.LocalAddr()
		if given, expected := addr.IP.String(), net.IPv6zero.String(); given != expected {
			t.Fatalf("given=%s expected=%s", given, expected)
		}
		if addr.Port == 0 {
			t.Fatal("port should not be 0")
		}
	}
	{
		peer, err := NewUDPPeer("udp6", "[::1]:0")
		if err != nil {
			t.Fatal(err)
		}
		defer peer.Close()

		addr := peer.LocalAddr()
		if given, expected := addr.IP.String(), net.IPv6loopback.String(); given != expected {
			t.Fatalf("given=%s expected=%s", given, expected)
		}
		if addr.Port == 0 {
			t.Fatal("port should not be 0")
		}
	}
	{
		peer, err := NewUDPPeer("udp6", "localhost:0")
		if err != nil {
			t.Fatal(err)
		}
		defer peer.Close()

		addr := peer.LocalAddr()
		if given, expected := addr.IP.String(), net.IPv6loopback.String(); given != expected {
			t.Fatalf("given=%s expected=%s", given, expected)
		}
		if addr.Port == 0 {
			t.Fatal("port should not be 0")
		}
	}
}

func TestUDPPeer_JoinInvalidGroup(t *testing.T) {
	peer, err := NewUDPPeer("udp", "")
	if err != nil {
		t.Fatal(err)
	}
	defer peer.Close()

	if err := peer.Join("0.0.0.0:4555"); err == nil {
		t.Fatal("should not have joined")
	}
}

func TestUDPPeer_Join(t *testing.T) {
	peer, err := NewUDPPeer("udp", "")
	if err != nil {
		t.Fatal(err)
	}
	defer peer.Close()

	if err := peer.Join("224.0.0.0"); err != nil {
		t.Fatal(err)
	}

	addr, err := ipv4.GetMulticastInterface(peer.socket)
	if err != nil {
		t.Fatal(err)
	}
	if !addr.IsUnspecified() {
		t.Fatal("multicast address should be unspecified")
	}
}

func TestUDPPeer_SetOutboundInterface1(t *testing.T) {
	peer, err := NewUDPPeer("udp", "")
	if err != nil {
		t.Fatal(err)
	}
	defer peer.Close()

	if err := peer.SetOutboundIPv4("en0"); err != nil {
		t.Fatal(err)
	}

	outIface, _ := peer.Outbound()
	if outIface == nil {
		t.Fatal("outbound interface should not be nil as its been explicitly set")
	}
}

func TestUDPPeer_SetOutboundInterface2(t *testing.T) {
	peer, err := NewUDPPeer("udp", "localhost:0")
	if err != nil {
		t.Fatal(err)
	}
	defer peer.Close()

	if err := peer.SetOutboundIPv4("en0"); err != nil {
		t.Fatal(err)
	}

	outIface, _ := peer.Outbound()
	if outIface == nil {
		t.Fatal("outbound interface should not be nil as its been explicitly set")
	}
}
