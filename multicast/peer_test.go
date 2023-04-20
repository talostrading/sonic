package multicast

import (
	"net"
	"testing"
)

func TestUDPPeer_IPv4Addresses(t *testing.T) {
	{
		_, err := NewUDPPeer("udp", net.IPv4zero.String())
		if err == nil {
			t.Fatal("should have received an error as the address is missing the port")
		}
	}
	{
		_, err := NewUDPPeer("udp4", net.IPv4zero.String())
		if err == nil {
			t.Fatal("should have received an error as the address is missing the port")
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
