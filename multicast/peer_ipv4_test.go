package multicast

import (
	"encoding/binary"
	"fmt"
	"log"
	"net"
	"net/netip"
	"sort"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/csdenboer/sonic"
	"github.com/csdenboer/sonic/net/ipv4"
	"github.com/csdenboer/sonic/sonicerrors"
)

// Listing multicast group memberships: netstat -gsv

func TestUDPPeerIPv4_Addresses(t *testing.T) {
	if len(testInterfacesIPv4) == 0 {
		return
	}

	ioc := sonic.MustIO()
	defer ioc.Close()

	{
		_, err := NewUDPPeer(ioc, "udp", net.IPv4zero.String())
		if err == nil {
			t.Fatal("should have received an error as the address is missing the port")
		}
	}
	{
		_, err := NewUDPPeer(ioc, "udp4", net.IPv4zero.String())
		if err == nil {
			t.Fatal("should have received an error as the address is missing the port")
		}
	}
	{
		peer, err := NewUDPPeer(ioc, "udp", "")
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
		peer, err := NewUDPPeer(ioc, "udp4", "")
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
		peer, err := NewUDPPeer(ioc, "udp", ":0")
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
		peer, err := NewUDPPeer(ioc, "udp4", ":0")
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
		peer, err := NewUDPPeer(ioc, "udp", "127.0.0.1:0")
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
		peer, err := NewUDPPeer(ioc, "udp4", "127.0.0.1:0")
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
		peer, err := NewUDPPeer(ioc, "udp", "localhost:0")
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
		peer, err := NewUDPPeer(ioc, "udp4", "localhost:0")
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

	log.Println("ran")
}

func TestUDPPeerIPv4_BindToInterfaceIP(t *testing.T) {
	xs, err := interfacesWithIP(4)
	if err == ErrNoInterfaces {
		log.Printf("skipping this test as no interfaces are available")
		return
	}
	if err != nil {
		t.Fatal(err)
	}

	ioc := sonic.MustIO()
	defer ioc.Close()
	for _, x := range xs {
		peer, err := NewUDPPeer(ioc, "udp", fmt.Sprintf("%s:0", x.ip.String()))
		if err != nil {
			t.Fatal(err)
		}
		peer.Close()
		log.Printf("bound peer to %s on %s", x.ip.String(), x.iff.Name)
	}
}

func TestUDPPeerIPv4_BindToMulticastIP(t *testing.T) {
	ioc := sonic.MustIO()
	defer ioc.Close()

	ip, err := netip.ParseAddr("224.0.0.0")
	if err != nil {
		t.Fatal(err)
	}
	if !ip.IsMulticast() {
		t.Fatal("ip should be multicast")
	}

	peer, err := NewUDPPeer(ioc, "udp", fmt.Sprintf("%s:0", ip.String()))
	if err != nil {
		t.Fatal(err)
	}
	peer.Close()
	log.Printf("bound peer to %s", ip.String())
}

func TestUDPPeerIPv4_JoinInvalidGroup(t *testing.T) {
	if len(testInterfacesIPv4) == 0 {
		return
	}

	ioc := sonic.MustIO()
	defer ioc.Close()

	peer, err := NewUDPPeer(ioc, "udp", "")
	if err != nil {
		t.Fatal(err)
	}
	defer peer.Close()

	if err := peer.Join("0.0.0.0:4555"); err == nil {
		t.Fatal("should not have joined")
	}

	log.Println("ran")
}

func TestUDPPeerIPv4_Join(t *testing.T) {
	if len(testInterfacesIPv4) == 0 {
		return
	}

	ioc := sonic.MustIO()
	defer ioc.Close()

	peer, err := NewUDPPeer(ioc, "udp", "")
	if err != nil {
		t.Fatal(err)
	}
	defer peer.Close()

	if err := peer.Join("224.0.0.0"); err != nil {
		t.Fatal(err)
	}

	addr, err := ipv4.GetMulticastInterfaceAddr(peer.socket)
	if err != nil {
		t.Fatal(err)
	}
	if !addr.IsUnspecified() {
		t.Fatal("multicast address should be unspecified")
	}

	log.Println("ran")
}

func TestUDPPeerIPv4_JoinOn(t *testing.T) {
	if len(testInterfacesIPv4) == 0 {
		return
	}

	ioc := sonic.MustIO()
	defer ioc.Close()

	peer, err := NewUDPPeer(ioc, "udp", "")
	if err != nil {
		t.Fatal(err)
	}
	defer peer.Close()

	if err := peer.JoinOn(
		"224.0.0.0", InterfaceName(testInterfacesIPv4[0].Name),
	); err != nil {
		t.Fatal(err)
	}

	addr, err := ipv4.GetMulticastInterfaceAddr(peer.socket)
	if err != nil {
		t.Fatal(err)
	}
	if !addr.IsUnspecified() {
		t.Fatal("multicast address should be unspecified")
	}

	log.Println("ran")
}

func TestUDPPeerIPv4_JoinSource(t *testing.T) {
	iffs, err := interfacesWithIP(4)
	if err == ErrNoInterfaces {
		log.Printf("skipping test as no interfaces are available")
		return
	}
	if err != nil {
		t.Fatal(err)
	}

	ioc := sonic.MustIO()
	defer ioc.Close()

	peer, err := NewUDPPeer(ioc, "udp", "")
	if err != nil {
		t.Fatal(err)
	}
	defer peer.Close()

	if err := peer.JoinSource(
		"224.0.0.0",
		SourceIP(iffs[0].ip.String()),
	); err != nil {
		t.Fatal(err)
	}

	addr, err := ipv4.GetMulticastInterfaceAddr(peer.socket)
	if err != nil {
		t.Fatal(err)
	}
	if !addr.IsUnspecified() {
		t.Fatal("multicast address should be unspecified")
	}

	log.Println("ran")
}

func TestUDPPeerIPv4_JoinSourceOn(t *testing.T) {
	iffs, err := interfacesWithIP(4)
	if err == ErrNoInterfaces {
		log.Printf("skipping test as no interfaces are available")
		return
	}
	if err != nil {
		t.Fatal(err)
	}

	ioc := sonic.MustIO()
	defer ioc.Close()

	peer, err := NewUDPPeer(ioc, "udp", "")
	if err != nil {
		t.Fatal(err)
	}
	defer peer.Close()

	if err := peer.JoinSourceOn(
		"224.0.0.0",
		SourceIP(iffs[0].ip.String()),
		InterfaceName(testInterfacesIPv4[0].Name),
	); err != nil {
		t.Fatal(err)
	}

	addr, err := ipv4.GetMulticastInterfaceAddr(peer.socket)
	if err != nil {
		t.Fatal(err)
	}
	if !addr.IsUnspecified() {
		t.Fatal("multicast address should be unspecified")
	}

	log.Println("ran")
}

func TestUDPPeerIPv4_SetLoop1(t *testing.T) {
	if len(testInterfacesIPv4) == 0 {
		return
	}

	ioc := sonic.MustIO()
	defer ioc.Close()

	peer, err := NewUDPPeer(ioc, "udp", "localhost:0")
	if err != nil {
		t.Fatal(err)
	}
	defer peer.Close()

	if peer.Loop() {
		t.Fatal("peer should not loop packets by default")
	}

	if err := peer.SetLoop(false); err != nil {
		t.Fatal(err)
	}
	if peer.Loop() {
		t.Fatal("peer should not loop packets")
	}

	if err := peer.SetLoop(true); err != nil {
		t.Fatal(err)
	}
	if !peer.Loop() {
		t.Fatal("peer should loop packets")
	}

	if err := peer.SetLoop(false); err != nil {
		t.Fatal(err)
	}
	if peer.Loop() {
		t.Fatal("peer should not loop packets")
	}

	log.Println("ran")
}

func TestUDPPeerIPv4_DefaultOutboundInterface(t *testing.T) {
	ioc := sonic.MustIO()
	defer ioc.Close()

	peer, err := NewUDPPeer(ioc, "udp", "")
	if err != nil {
		t.Fatal(err)
	}
	defer peer.Close()

	fmt.Println(peer.Outbound())
}

func TestUDPPeerIPv4_SetOutboundInterfaceOnUnspecifiedIPAndPort(t *testing.T) {
	if len(testInterfacesIPv4) == 0 {
		return
	}

	ioc := sonic.MustIO()
	defer ioc.Close()

	peer, err := NewUDPPeer(ioc, "udp", "")
	if err != nil {
		t.Fatal(err)
	}
	defer peer.Close()

	for _, iff := range testInterfacesIPv4 {
		fmt.Println("setting", iff.Name, "as outbound")

		if err := peer.SetOutboundIPv4(iff.Name); err != nil {
			t.Fatal(err)
		}

		outboundInterface, outboundIP := peer.Outbound()
		fmt.Println("outbound for", iff.Name, outboundInterface, outboundIP)

		{
			addr, err := ipv4.GetMulticastInterfaceAddr(peer.NextLayer())
			if err != nil {
				t.Fatal(err)
			}
			fmt.Printf(
				"%s GetMulticastInterface_Inet4Addr addr=%s\n",
				iff.Name, addr.String())
		}

		{
			interfaceAddr, multicastAddr, err := ipv4.GetMulticastInterfaceAddrAndGroup(peer.NextLayer())
			if err != nil {
				t.Fatal(err)
			}
			fmt.Printf(
				"%s GetMulticastInterface_IPMreq4Addr interface_addr=%s multicast_addr=%s\n",
				iff.Name, interfaceAddr.String(), multicastAddr.String())
		}

		{
			interfaceIndex, err := ipv4.GetMulticastInterfaceIndex(peer.NextLayer())
			if err != nil {
				t.Fatal(err)
			}
			fmt.Printf(
				"%s GetMulticastInterface_Index interface_index=%d\n",
				iff.Name, interfaceIndex)
		}
	}

	log.Println("ran")
}

func TestUDPPeerIPv4_SetOutboundInterfaceOnUnspecifiedPort(t *testing.T) {
	if len(testInterfacesIPv4) == 0 {
		return
	}

	ioc := sonic.MustIO()
	defer ioc.Close()

	peer, err := NewUDPPeer(ioc, "udp", "localhost:0")
	if err != nil {
		t.Fatal(err)
	}
	defer peer.Close()

	for _, iff := range testInterfacesIPv4 {
		fmt.Println("setting", iff.Name, "as outbound")

		if err := peer.SetOutboundIPv4(iff.Name); err != nil {
			t.Fatal(err)
		}

		outboundInterface, outboundIP := peer.Outbound()
		fmt.Println("outbound for", iff.Name, outboundInterface, outboundIP)

		{
			addr, err := ipv4.GetMulticastInterfaceAddr(peer.NextLayer())
			if err != nil {
				t.Fatal(err)
			}
			fmt.Printf(
				"%s GetMulticastInterface_Inet4Addr addr=%s\n",
				iff.Name, addr.String())
		}

		{
			interfaceAddr, multicastAddr, err := ipv4.GetMulticastInterfaceAddrAndGroup(peer.NextLayer())
			if err != nil {
				t.Fatal(err)
			}
			fmt.Printf(
				"%s GetMulticastInterface_IPMreq4Addr interface_addr=%s multicast_addr=%s\n",
				iff.Name, interfaceAddr.String(), multicastAddr.String())
		}

		{
			interfaceIndex, err := ipv4.GetMulticastInterfaceIndex(peer.NextLayer())
			if err != nil {
				t.Fatal(err)
			}
			fmt.Printf(
				"%s GetMulticastInterface_Index interface_index=%d\n",
				iff.Name, interfaceIndex)
		}
	}

	log.Println("ran")
}

func TestUDPPeerIPv4_TTL(t *testing.T) {
	if len(testInterfacesIPv4) == 0 {
		return
	}

	ioc := sonic.MustIO()
	defer ioc.Close()

	peer, err := NewUDPPeer(ioc, "udp", "")
	if err != nil {
		t.Fatal(err)
	}

	actualTTL, err := ipv4.GetMulticastTTL(peer.NextLayer())
	if err != nil {
		t.Fatal(err)
	}

	if actualTTL != peer.TTL() {
		t.Fatalf("wrong TTL expected=%d given=%d", actualTTL, peer.TTL())
	}

	if peer.TTL() != 1 {
		t.Fatalf("peer TTL should be 1 by default")
	}

	setAndCheck := func(ttl uint8) {
		if err := peer.SetTTL(ttl); err != nil {
			t.Fatal(err)
		}

		if peer.TTL() != ttl {
			t.Fatalf("peer TTL should be %d", ttl)
		}
	}

	for i := 0; i <= 255; i++ {
		setAndCheck(uint8(i))
	}

	log.Println("ran")
}

func TestUDPPeerIPv4_Reader1(t *testing.T) {
	// 1 reader on INADDR_ANY joining 224.0.0.0, 1 writer on
	// 224.0.0.0:<reader_port>

	r := newTestRW(t, "udp", "")
	defer r.Close()

	multicastIP := "224.0.0.0"
	multicastPort := r.peer.LocalAddr().Port
	multicastAddr := fmt.Sprintf("%s:%d", multicastIP, multicastPort)
	if err := r.peer.Join(IP(multicastIP)); err != nil {
		t.Fatal(err)
	}

	w := newTestRW(t, "udp", "")
	defer w.Close()

	var wg sync.WaitGroup
	wg.Add(2)

	start := time.Now()
	go func() {
		defer wg.Done()

		count := 0
		r.ReadLoop(func(err error, _ uint64, _ netip.AddrPort) {
			if err != nil {
				panic(err)
			} else {
				count++
				if count == 10 ||
					// just to not have it hang
					time.Since(start).Seconds() > 1 {
					r.Close()
				}
			}
		})
	}()

	go func() {
		defer wg.Done()
		for i := 0; i < 10; i++ {
			if err := w.WriteNext(multicastAddr); err != nil &&
				err != sonicerrors.ErrNoBufferSpaceAvailable {
				panic(err)
			}
			time.Sleep(time.Millisecond)
		}
	}()

	wg.Wait()

	if len(r.ReceivedFrom()) != 1 {
		t.Fatal("should have received from exactly one source")
	}

	fmt.Println(r.received)
}

func TestUDPPeerIPv4_Reader2(t *testing.T) {
	// 1 reader on INADDR_ANY.
	// 2 writers on multicastAddr: 224.0.0.1:<reader_port>.
	// reader joins 224.0.0.1. Reader should get from both.

	r := newTestRW(t, "udp", "")
	defer r.Close()

	multicastIP := "224.0.0.1"
	multicastPort := r.peer.LocalAddr().Port
	multicastAddr := fmt.Sprintf("%s:%d", multicastIP, multicastPort)
	if err := r.peer.Join(IP(multicastIP)); err != nil {
		t.Fatal(err)
	}

	w1 := newTestRW(t, "udp", "")
	defer w1.Close()
	w2 := newTestRW(t, "udp", "")
	defer w2.Close()

	var wg sync.WaitGroup
	wg.Add(3)

	start := time.Now()
	go func() {
		defer wg.Done()

		count := make(map[netip.AddrPort]int)
		r.ReadLoop(func(err error, _ uint64, from netip.AddrPort) {
			if err != nil {
				panic(err)
			} else {
				count[from]++
				stopCount := 0
				for _, c := range count {
					if c == 10 {
						stopCount++
					}
				}

				if stopCount == 2 ||
					// just to not have it hang
					time.Since(start).Seconds() > 1 {
					r.Close()
				}
			}
		})
	}()

	go func() {
		defer wg.Done()
		for i := 0; i < 10; i++ {
			if err := w1.WriteNext(multicastAddr); err != nil &&
				err != sonicerrors.ErrNoBufferSpaceAvailable {
				panic(err)
			}
			time.Sleep(time.Millisecond)
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 10; i++ {
			if err := w2.WriteNext(multicastAddr); err != nil &&
				err != sonicerrors.ErrNoBufferSpaceAvailable {
				panic(err)
			}
			time.Sleep(time.Millisecond)
		}
	}()

	wg.Wait()

	if len(r.ReceivedFrom()) != 2 {
		t.Fatal("should have received from exactly two sources")
	}

	fmt.Println(r.received)
}

func TestUDPPeerIPv4_Reader3(t *testing.T) {
	// 1 reader on INADDR_ANY.
	// 2 writers:
	// - one on 224.0.0.2:<reader_port>
	// - two on 224.0.0.3:<reader_port>
	// The reader only joins 224.0.0.2. Should only get from writer 1.

	r := newTestRW(t, "udp", "")
	defer r.Close()

	multicastIP1 := "224.0.0.2"
	multicastAddr1 := fmt.Sprintf(
		"%s:%d", multicastIP1, r.peer.LocalAddr().Port)
	if err := r.peer.Join(IP(multicastIP1)); err != nil {
		t.Fatal(err)
	}

	multicastIP2 := "224.0.0.3"
	multicastAddr2 := fmt.Sprintf(
		"%s:%d", multicastIP2, r.peer.LocalAddr().Port)

	w1 := newTestRW(t, "udp", "")
	defer w1.Close()
	w2 := newTestRW(t, "udp", "")
	defer w2.Close()

	var wg sync.WaitGroup
	wg.Add(3)

	start := time.Now()
	go func() {
		defer wg.Done()

		count := make(map[netip.AddrPort]int)
		r.ReadLoop(func(err error, _ uint64, from netip.AddrPort) {
			if err != nil {
				panic(err)
			} else {
				count[from]++
				stopCount := 0
				for _, c := range count {
					if c == 10 {
						stopCount++
					}
				}

				if stopCount == 1 ||
					// just to not have it hang
					time.Since(start).Seconds() > 1 {
					r.Close()
				}
			}
		})
	}()

	go func() {
		defer wg.Done()
		for i := 0; i < 10; i++ {
			if err := w1.WriteNext(multicastAddr1); err != nil &&
				err != sonicerrors.ErrNoBufferSpaceAvailable {
				panic(err)
			}
			time.Sleep(time.Millisecond)
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 10; i++ {
			if err := w2.WriteNext(multicastAddr2); err != nil &&
				err != sonicerrors.ErrNoBufferSpaceAvailable {
				panic(err)
			}
			time.Sleep(time.Millisecond)
		}
	}()

	wg.Wait()

	if len(r.ReceivedFrom()) != 1 {
		t.Fatal("should have received from exactly one source")
	}

	fmt.Println(r.received)
}

func TestUDPPeerIPv4_Reader4(t *testing.T) {
	// 1 reader on INADDR_ANY.
	// 2 writers:
	// - one on 224.0.0.4:<reader_port>
	// - two on 224.0.0.4:<not_reader_port>
	// The reader only joins 224.0.0.4. Should only get from writer 1 since
	// writer 2 does not publish on the reader's port.

	r := newTestRW(t, "udp", "")
	defer r.Close()

	multicastIP1 := "224.0.0.4"
	multicastAddr1 := fmt.Sprintf(
		"%s:%d", multicastIP1, r.peer.LocalAddr().Port)
	if err := r.peer.Join(IP(multicastIP1)); err != nil {
		t.Fatal(err)
	}

	multicastAddr2 := fmt.Sprintf(
		"%s:%d", multicastIP1, r.peer.LocalAddr().Port+1)

	w1 := newTestRW(t, "udp", "")
	defer w1.Close()
	w2 := newTestRW(t, "udp", "")
	defer w2.Close()

	var wg sync.WaitGroup
	wg.Add(3)

	start := time.Now()
	go func() {
		defer wg.Done()

		count := make(map[netip.AddrPort]int)
		r.ReadLoop(func(err error, _ uint64, from netip.AddrPort) {
			if err != nil {
				panic(err)
			} else {
				count[from]++
				stopCount := 0
				for _, c := range count {
					if c == 10 {
						stopCount++
					}
				}

				if stopCount == 1 ||
					// just to not have it hang
					time.Since(start).Seconds() > 1 {
					r.Close()
				}
			}
		})
	}()

	go func() {
		defer wg.Done()
		for i := 0; i < 10; i++ {
			if err := w1.WriteNext(multicastAddr1); err != nil &&
				err != sonicerrors.ErrNoBufferSpaceAvailable {
				panic(err)
			}
			time.Sleep(time.Millisecond)
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 10; i++ {
			if err := w2.WriteNext(multicastAddr2); err != nil &&
				err != sonicerrors.ErrNoBufferSpaceAvailable {
				panic(err)
			}
			time.Sleep(time.Millisecond)
		}
	}()

	wg.Wait()

	if len(r.ReceivedFrom()) != 1 {
		t.Fatal("should have received from exactly one source")
	}

	fmt.Println(r.received)
}

func TestUDPPeerIPv4_Reader5(t *testing.T) {
	// Counterpart to TestReader2.
	//
	// 1 reader on 224.0.0.5:0(so random port). Joins both 224.0.0.5 and
	// 224.0.0.6.
	//
	// 2 writers:
	// - one on 224.0.0.5:<reader_port>
	// - two on 224.0.0.6:<not_reader_port>
	//
	// The reader joins both groups, but it's bound to 224.0.0.5, which has a
	// filtering role, meaning that it should only get from 224.0.0.5 and not
	// also from 224.0.0.6.

	r := newTestRW(t, "udp", "224.0.0.5:0")
	defer r.Close()

	multicastIP1 := "224.0.0.5"
	multicastAddr1 := fmt.Sprintf("%s:%d", multicastIP1, r.peer.LocalAddr().Port)
	if err := r.peer.Join(IP(multicastIP1)); err != nil {
		t.Fatal(err)
	}

	multicastIP2 := "224.0.0.6"
	multicastAddr2 := fmt.Sprintf("%s:%d", multicastIP2, r.peer.LocalAddr().Port)
	if err := r.peer.Join(IP(multicastIP2)); err != nil {
		t.Fatal(err)
	}

	w1 := newTestRW(t, "udp", "")
	defer w1.Close()
	w2 := newTestRW(t, "udp", "")
	defer w2.Close()

	var wg sync.WaitGroup
	wg.Add(3)

	start := time.Now()
	go func() {
		defer wg.Done()

		count := make(map[netip.AddrPort]int)
		r.ReadLoop(func(err error, _ uint64, from netip.AddrPort) {
			if err != nil {
				panic(err)
			} else {
				count[from]++
				stopCount := 0
				for _, c := range count {
					if c == 10 {
						stopCount++
					}
				}

				if stopCount == 1 ||
					// just to not have it hang
					time.Since(start).Seconds() > 1 {
					r.Close()
				}
			}
		})
	}()

	go func() {
		defer wg.Done()
		for i := 0; i < 10; i++ {
			if err := w1.WriteNext(multicastAddr1); err != nil &&
				err != sonicerrors.ErrNoBufferSpaceAvailable {
				panic(err)
			}
			time.Sleep(time.Millisecond)
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 10; i++ {
			if err := w2.WriteNext(multicastAddr2); err != nil &&
				err != sonicerrors.ErrNoBufferSpaceAvailable {
				panic(err)
			}
			time.Sleep(time.Millisecond)
		}
	}()

	wg.Wait()

	if len(r.ReceivedFrom()) != 1 {
		t.Fatal("should have received from exactly one source")
	}

	fmt.Println(r.received)
}

func TestUDPPeerIPv4_Reader6(t *testing.T) {
	// 1 reader on localhost:0(so random port). Joins 224.0.0.7.
	// 1 writer bound to 0.0.0.0:0 and sending on 224.0.0.7:<reader_port>.
	// Reader should not receive anything because it is bound to a unicast IP.

	r := newTestRW(t, "udp", "localhost:0")
	defer r.Close()

	multicastIP := "224.0.0.7"
	multicastAddr := fmt.Sprintf("%s:%d", multicastIP, r.peer.LocalAddr().Port)
	if err := r.peer.Join(IP(multicastIP)); err != nil {
		t.Fatal(err)
	}

	w := newTestRW(t, "udp", "")
	defer w.Close()

	readerGot := 0
	go func() {
		r.ReadLoop(func(err error, _ uint64, _ netip.AddrPort) {
			if err != nil {
				panic(err)
			} else {
				readerGot++
			}
		})
	}()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 10; i++ {
			if err := w.WriteNext(multicastAddr); err != nil &&
				err != sonicerrors.ErrNoBufferSpaceAvailable {
				panic(err)
			}
			time.Sleep(time.Millisecond)
		}
	}()

	wg.Wait()

	if readerGot != 0 {
		t.Fatal("reader should have not received anything")
	}

	if len(r.ReceivedFrom()) != 0 {
		t.Fatal("should have received from no sources")
	}

	fmt.Println(r.received)
}

func TestUDPPeer_SetInbound1(t *testing.T) {
	if len(testInterfacesIPv4) < 1 {
		log.Printf(
			"not running this one as we don't have enough multicast interfaces")
		for _, iff := range testInterfacesIPv4 {
			log.Printf("interface name=%s", iff.Name)
		}
		return
	}

	readerInterface := testInterfacesIPv4[0]
	writerInterface := testInterfacesIPv4[0]
	log.Printf(
		"reader_interface=%s writer_interface=%s",
		readerInterface.Name, writerInterface.Name)

	r := newTestRW(t, "udp", "")
	defer r.Close()
	if err := r.peer.SetInbound(readerInterface.Name); err != nil {
		t.Fatal(err)
	}
	if err := r.peer.Join("224.0.0.8"); err != nil {
		t.Fatal(err)
	}

	w := newTestRW(t, "udp", "")
	defer w.Close()
	if err := r.peer.SetOutboundIPv4(writerInterface.Name); err != nil {
		t.Fatal(err)
	}

	multicastAddr := fmt.Sprintf("224.0.0.8:%d", r.peer.LocalAddr().Port)

	var count int32 = 0
	go func() {
		r.ReadLoop(func(_ error, _ uint64, _ netip.AddrPort) {
			atomic.AddInt32(&count, 1)
		})
	}()

	time.Sleep(time.Millisecond)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 50; i++ {
			if err := w.WriteNext(multicastAddr); err != nil &&
				err != sonicerrors.ErrNoBufferSpaceAvailable {
				panic(err)
			}
			time.Sleep(time.Millisecond)
		}
	}()

	wg.Wait()

	if atomic.LoadInt32(&count) == 0 {
		t.Fatal("reader should have received something")
	}
	log.Printf("reader received %d", count)
}

func TestUDPPeer_SetInbound2(t *testing.T) {
	// TODO warning - this test is not correct. We explicitly tell the kernel to
	// not loopback multicast packets. SetInbound might not be correct but I
	// need a proper setup to test that.

	if len(testInterfacesIPv4) < 2 {
		log.Printf(
			"not running this one as we don't have enough multicast interfaces")
		for _, iff := range testInterfacesIPv4 {
			log.Printf("interface name=%s", iff.Name)
		}
		return
	}

	readerInterface := testInterfacesIPv4[0]
	writerInterface := testInterfacesIPv4[1]
	log.Printf(
		"reader_interface=%s writer_interface=%s",
		readerInterface.Name, writerInterface.Name)

	r := newTestRW(t, "udp", "")
	defer r.Close()
	if err := r.peer.SetInbound(readerInterface.Name); err != nil {
		t.Fatal(err)
	}
	if err := r.peer.Join("224.0.0.9"); err != nil {
		t.Fatal(err)
	}
	if err := r.peer.SetLoop(false); err != nil {
		t.Fatal(err)
	}

	w := newTestRW(t, "udp", "")
	defer w.Close()
	if err := r.peer.SetOutboundIPv4(writerInterface.Name); err != nil {
		t.Fatal(err)
	}
	if err := w.peer.SetLoop(false); err != nil {
		t.Fatal(err)
	}

	multicastAddr := fmt.Sprintf("224.0.0.9:%d", r.peer.LocalAddr().Port)

	var count int32 = 0
	go func() {
		r.ReadLoop(func(_ error, _ uint64, _ netip.AddrPort) {
			atomic.AddInt32(&count, 1)
		})
	}()

	time.Sleep(time.Millisecond)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 50; i++ {
			if err := w.WriteNext(multicastAddr); err != nil &&
				err != sonicerrors.ErrNoBufferSpaceAvailable {
				panic(err)
			}
			time.Sleep(time.Millisecond)
		}
	}()

	wg.Wait()

	if atomic.LoadInt32(&count) != 0 {
		t.Fatal("reader should have not received anything")
	}
}

func TestUDPPeerIPv4_JoinAndRead(t *testing.T) {
	r := newTestRW(t, "udp", "224.0.0.10:0")
	w := newTestRW(t, "udp", "")

	w.peer.LocalAddr()
	if err := r.peer.Join("224.0.0.10"); err != nil {
		t.Fatal(err)
	}

	var count int32
	go func() {
		r.ReadLoop(func(err error, _ uint64, _ netip.AddrPort) {
			if err == nil {
				atomic.AddInt32(&count, 1)
			}
		})
	}()

	multicastAddr := fmt.Sprintf("224.0.0.10:%d", r.peer.LocalAddr().Port)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()

		for i := 0; i < 10; i++ {
			if err := w.WriteNext(multicastAddr); err != nil &&
				err != sonicerrors.ErrNoBufferSpaceAvailable {
				panic(err)
			}
			time.Sleep(time.Millisecond)
		}
	}()
	wg.Wait()

	if atomic.LoadInt32(&count) == 0 {
		t.Fatal("reader did not read anything")
	}
}

func TestUDPPeerIPv4_JoinOnAndRead(t *testing.T) {
	iffs, err := interfacesWithIP(4)
	if err == ErrNoInterfaces {
		log.Printf("skipping this test as not interfaces are available")
		return
	}

	r := newTestRW(t, "udp", "224.0.0.11:0")
	w := newTestRW(t, "udp", fmt.Sprintf("%s:0", iffs[0].ip))

	if w.peer.LocalAddr().IP.String() != iffs[0].ip.String() {
		t.Fatal("something wrong with binding to an interface address")
	}

	w.peer.LocalAddr()
	if err := r.peer.JoinOn(
		"224.0.0.11",
		InterfaceName(iffs[0].iff.Name),
	); err != nil {
		t.Fatal(err)
	}

	var count int32
	go func() {
		r.ReadLoop(func(err error, _ uint64, _ netip.AddrPort) {
			if err == nil {
				atomic.AddInt32(&count, 1)
			}
		})
	}()

	multicastAddr := fmt.Sprintf("224.0.0.11:%d", r.peer.LocalAddr().Port)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()

		for i := 0; i < 10; i++ {
			if err := w.WriteNext(multicastAddr); err != nil &&
				err != sonicerrors.ErrNoBufferSpaceAvailable {
				panic(err)
			}
			time.Sleep(time.Millisecond)
		}
	}()
	wg.Wait()

	if atomic.LoadInt32(&count) == 0 {
		t.Fatal("reader did not read anything")
	}
}

func TestUDPPeerIPv4_JoinReadLeave(t *testing.T) {
	ioc := sonic.MustIO()
	defer ioc.Close()

	multicastIP := "224.0.0.12"
	r, err := NewUDPPeer(ioc, "udp", fmt.Sprintf("%s:0", multicastIP))
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()

	log.Printf("reader local_addr=%s", r.LocalAddr())

	multicastAddr, err := netip.ParseAddrPort(
		fmt.Sprintf("%s:%d", multicastIP, r.LocalAddr().Port))
	if err != nil {
		t.Fatal(err)
	}

	if err := r.Join(IP(multicastIP)); err != nil {
		t.Fatal(err)
	}

	var (
		onRead        func(error, int, netip.AddrPort)
		rb            = make([]byte, 128)
		nReadBefore   = 0
		nReadAfter    = 0
		nTotal        = 0
		lastReadAfter time.Time
		left          = false
	)
	onRead = func(err error, _ int, from netip.AddrPort) {
		if err == nil {
			nTotal++
			if !left {
				if nReadBefore == 0 {
					log.Printf("reader receiving from %s", from)
				}
				nReadBefore++
			} else {
				nReadAfter++
				lastReadAfter = time.Now()
			}

			if !left && nReadBefore >= 5 {
				log.Printf("leaving multicast group")
				if err := r.Leave(IP(multicastIP)); err != nil {
					t.Fatal(err)
				}
				left = true
			}
		} else {
			log.Printf("err=%v", err)
		}
		r.AsyncRead(rb, onRead)
	}
	r.AsyncRead(rb, onRead)

	w, err := NewUDPPeer(ioc, "udp", fmt.Sprintf("%s:0", multicastIP))
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()

	var (
		onWrite func(error, int)
		wb      = []byte("hello")
	)
	onWrite = func(err error, _ int) {
		if err == nil {
			w.AsyncWrite(wb, multicastAddr, onWrite)
		} else {
			log.Printf("err=%v", err)
		}
	}
	w.AsyncWrite(wb, multicastAddr, onWrite)

	log.Printf("writer local_addr=%s", w.LocalAddr())

	start := time.Now()
	var now time.Time
	for {
		now = time.Now()
		if now.Sub(start).Seconds() < 10 {
			ioc.PollOne()
		} else {
			break
		}
	}
	log.Printf(
		"done before=%d after=%d total=%d rem=%d now-last_read_after_block=%s",
		nReadBefore,
		nReadAfter,
		nTotal,
		nTotal-nReadBefore-nReadAfter,
		now.Sub(lastReadAfter))
}

func TestUDPPeerIPv4_JoinReadBlock(t *testing.T) {
	ioc := sonic.MustIO()
	defer ioc.Close()

	multicastIP := "224.0.0.13"
	r, err := NewUDPPeer(ioc, "udp", fmt.Sprintf("%s:0", multicastIP))
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()

	log.Printf("reader local_addr=%s", r.LocalAddr())

	multicastAddr, err := netip.ParseAddrPort(
		fmt.Sprintf("%s:%d", multicastIP, r.LocalAddr().Port))
	if err != nil {
		t.Fatal(err)
	}

	if err := r.Join(IP(multicastIP)); err != nil {
		t.Fatal(err)
	}

	var (
		onRead        func(error, int, netip.AddrPort)
		rb            = make([]byte, 128)
		nReadBefore   = 0
		nReadAfter    = 0
		nTotal        = 0
		lastReadAfter time.Time
		left          = false
	)
	onRead = func(err error, _ int, from netip.AddrPort) {
		if err == nil {
			if !left {
				if nReadBefore == 0 {
					log.Printf("reader receiving from %s", from)
				}
				nReadBefore++
			} else {
				nReadAfter++
				lastReadAfter = time.Now()
			}

			if !left && nReadBefore >= 5 {
				log.Printf("blocking source %s", from.Addr())
				if err := r.BlockSource(
					IP(multicastIP),
					SourceIP(from.Addr().String()),
				); err != nil {
					t.Fatal(err)
				}
				left = true
			}
		} else {
			log.Printf("err=%v", err)
		}
		r.AsyncRead(rb, onRead)
	}
	r.AsyncRead(rb, onRead)

	w, err := NewUDPPeer(ioc, "udp", fmt.Sprintf("%s:0", multicastIP))
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()

	var (
		onWrite func(error, int)
		wb      = []byte("hello")
	)
	onWrite = func(err error, _ int) {
		if err == nil {
			nTotal++
			w.AsyncWrite(wb, multicastAddr, onWrite)
		} else {
			log.Printf("err=%v", err)
		}
	}
	w.AsyncWrite(wb, multicastAddr, onWrite)

	log.Printf("writer local_addr=%s", w.LocalAddr())

	start := time.Now()
	var now time.Time
	for {
		now = time.Now()
		if now.Sub(start).Seconds() < 10 {
			ioc.PollOne()
		} else {
			break
		}
	}

	rem := nTotal - nReadBefore - nReadAfter
	log.Printf(
		"done before=%d after=%d total=%d rem=%d now-last_read_after_block=%s",
		nReadBefore,
		nReadAfter,
		nTotal,
		rem,
		now.Sub(lastReadAfter))
	if rem <= 0 {
		t.Fatal("read everything after blocking the source")
	}
}

func TestUDPPeerIPv4_MultipleReadersOnINADDRANY_NoneJoin(t *testing.T) {
	ioc := sonic.MustIO()
	defer ioc.Close()

	multicastIP := "224.0.0.14"
	multicastPort := 1234
	multicastAddr, err := netip.ParseAddrPort(
		fmt.Sprintf("%s:%d", multicastIP, multicastPort))
	if err != nil {
		t.Fatal(err)
	}

	readers := make(map[*UDPPeer]struct {
		index int
		nRead int
		from  map[netip.AddrPort]struct{}
	})
	for i := 0; i < 10; i++ {
		r, err := NewUDPPeer(ioc, "udp", fmt.Sprintf(":%d", multicastPort))
		if err != nil {
			t.Fatal(err)
		}
		defer r.Close()

		if !r.LocalAddr().IP.IsUnspecified() {
			t.Fatal("reader should be on INADDR_ANY")
		}

		init := struct {
			index int
			nRead int
			from  map[netip.AddrPort]struct{}
		}{
			index: i,
			nRead: 0,
			from:  make(map[netip.AddrPort]struct{}),
		}
		readers[r] = init

		b := make([]byte, 128)
		var onRead func(error, int, netip.AddrPort)
		onRead = func(err error, _ int, from netip.AddrPort) {
			if err == nil {
				entry := readers[r]
				entry.nRead++
				entry.from[from] = struct{}{}
				readers[r] = entry

				r.AsyncRead(b, onRead)
			}
		}
		r.AsyncRead(b, onRead)
	}

	w, err := NewUDPPeer(ioc, "udp", "")
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()

	var onWrite func(error, int)
	onWrite = func(err error, _ int) {
		if err == nil {
			w.AsyncWrite([]byte("hello"), multicastAddr, onWrite)
		}
	}
	w.AsyncWrite([]byte("hello"), multicastAddr, onWrite)

	start := time.Now()
	now := time.Now()
	for now.Sub(start).Seconds() < 5 {
		now = time.Now()
		ioc.PollOne()
	}

	log.Printf("done")

	var errs []error
	for _, reader := range readers {
		log.Printf(
			"reader index=%d n_read=%d from=%+v",
			reader.index, reader.nRead, reader.from)
		if reader.nRead != 0 || len(reader.from) != 0 {
			errs = append(errs,
				fmt.Errorf(
					"none of the readers joined, but reader %d received %d times from %+v",
					reader.index, reader.nRead, reader.from))
		}
	}

	if len(errs) > 0 {
		t.Fatalf("errs=%+v", errs)
	}
}

// See also
// TestUDPPeerIPv4_MultipleReadersOnINADDRANY_OneJoins_IP_MULTICAST_ALL_is_1_default
// in peer_ipv4_linux_test.go
func TestUDPPeerIPv4_MultipleReadersOnINADDRANY_OneJoins(t *testing.T) {
	ioc := sonic.MustIO()
	defer ioc.Close()

	multicastIP := "224.0.0.15"
	multicastPort := 1234
	multicastAddr, err := netip.ParseAddrPort(
		fmt.Sprintf("%s:%d", multicastIP, multicastPort))
	if err != nil {
		t.Fatal(err)
	}

	readers := make(map[*UDPPeer]struct {
		index int
		nRead int
		from  map[netip.AddrPort]struct{}
	})
	for i := 0; i < 10; i++ {
		r, err := NewUDPPeer(ioc, "udp", fmt.Sprintf(":%d", multicastPort))
		if err != nil {
			t.Fatal(err)
		}
		defer r.Close()

		if !r.LocalAddr().IP.IsUnspecified() {
			t.Fatal("reader should be on INADDR_ANY")
		}

		if i == 0 {
			if err := r.Join(IP(multicastIP)); err != nil {
				t.Fatalf("reader could not join %s", multicastIP)
			} else {
				log.Printf("reader %d joined group %s", i, multicastIP)
			}
		}

		init := struct {
			index int
			nRead int
			from  map[netip.AddrPort]struct{}
		}{
			index: i,
			nRead: 0,
			from:  make(map[netip.AddrPort]struct{}),
		}
		readers[r] = init

		b := make([]byte, 128)
		var onRead func(error, int, netip.AddrPort)
		onRead = func(err error, _ int, from netip.AddrPort) {
			if err == nil {
				entry := readers[r]
				entry.nRead++
				entry.from[from] = struct{}{}
				readers[r] = entry

				r.AsyncRead(b, onRead)
			}
		}
		r.AsyncRead(b, onRead)
	}

	w, err := NewUDPPeer(ioc, "udp", "")
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()

	var onWrite func(error, int)
	onWrite = func(err error, _ int) {
		if err == nil {
			w.AsyncWrite([]byte("hello"), multicastAddr, onWrite)
		}
	}
	w.AsyncWrite([]byte("hello"), multicastAddr, onWrite)

	start := time.Now()
	now := time.Now()
	for now.Sub(start).Seconds() < 5 {
		now = time.Now()
		ioc.PollOne()
	}

	log.Printf("done")

	for _, reader := range readers {
		log.Printf(
			"reader index=%d n_read=%d from=%+v",
			reader.index, reader.nRead, reader.from)
		if reader.index != 0 /* the one that joined */ && reader.nRead != 0 {
			t.Fatal("only one the reader who joined should have read something")
		}
	}
}

func TestUDPPeerIPv4_MultipleReadersOnMulticast_OneJoins(t *testing.T) {
	ioc := sonic.MustIO()
	defer ioc.Close()

	multicastIP := "224.0.0.16"
	multicastPort := 1234
	multicastAddr, err := netip.ParseAddrPort(
		fmt.Sprintf("%s:%d", multicastIP, multicastPort))
	if err != nil {
		t.Fatal(err)
	}

	readers := make(map[*UDPPeer]struct {
		index int
		nRead int
		from  map[netip.AddrPort]struct{}
	})
	for i := 0; i < 10; i++ {
		r, err := NewUDPPeer(
			ioc,
			"udp",
			fmt.Sprintf("%s:%d", multicastIP, multicastPort),
		)
		if err != nil {
			t.Fatal(err)
		}
		defer r.Close()

		if r.LocalAddr().IP.IsUnspecified() {
			t.Fatal("reader should not be on INADDR_ANY")
		}

		if i == 0 {
			if err := r.Join(IP(multicastIP)); err != nil {
				t.Fatalf("reader could not join %s", multicastIP)
			} else {
				log.Printf("reader %d joined group %s", i, multicastIP)
			}
		}

		init := struct {
			index int
			nRead int
			from  map[netip.AddrPort]struct{}
		}{
			index: i,
			nRead: 0,
			from:  make(map[netip.AddrPort]struct{}),
		}
		readers[r] = init

		b := make([]byte, 128)
		var onRead func(error, int, netip.AddrPort)
		onRead = func(err error, _ int, from netip.AddrPort) {
			if err == nil {
				entry := readers[r]
				entry.nRead++
				entry.from[from] = struct{}{}
				readers[r] = entry

				r.AsyncRead(b, onRead)
			}
		}
		r.AsyncRead(b, onRead)
	}

	w, err := NewUDPPeer(ioc, "udp", "")
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()

	var onWrite func(error, int)
	onWrite = func(err error, _ int) {
		if err == nil {
			w.AsyncWrite([]byte("hello"), multicastAddr, onWrite)
		}
	}
	w.AsyncWrite([]byte("hello"), multicastAddr, onWrite)

	start := time.Now()
	now := time.Now()
	for now.Sub(start).Seconds() < 5 {
		now = time.Now()
		ioc.PollOne()
	}

	log.Printf("done")

	for r, reader := range readers {
		log.Printf("reader index=%d n_read=%d from=%+v local_addr=%s",
			reader.index, reader.nRead, reader.from, r.LocalAddr())
		if reader.index != 0 /* the one that joined */ && reader.nRead != 0 {
			t.Fatal("only one the reader who joined should have read something")
		}
	}
}

func TestUDPPeerIPv4_MultipleReadersOnMulticast_AllJoin(t *testing.T) {
	ioc := sonic.MustIO()
	defer ioc.Close()

	multicastIP := "224.0.0.17"
	multicastPort := 1234
	multicastAddr, err := netip.ParseAddrPort(
		fmt.Sprintf("%s:%d", multicastIP, multicastPort))
	if err != nil {
		t.Fatal(err)
	}

	readers := make(map[*UDPPeer]struct {
		index int
		nRead int
		from  map[netip.AddrPort]struct{}
	})
	for i := 0; i < 10; i++ {
		r, err := NewUDPPeer(ioc, "udp", fmt.Sprintf(
			"%s:%d", multicastIP, multicastPort))
		if err != nil {
			t.Fatal(err)
		}
		defer r.Close()

		if r.LocalAddr().IP.IsUnspecified() {
			t.Fatal("reader should not be on INADDR_ANY")
		}

		if err := r.Join(IP(multicastIP)); err != nil {
			t.Fatalf("reader could not join %s", multicastIP)
		} else {
			log.Printf("reader %d joined group %s", i, multicastIP)
		}

		init := struct {
			index int
			nRead int
			from  map[netip.AddrPort]struct{}
		}{
			index: i,
			nRead: 0,
			from:  make(map[netip.AddrPort]struct{}),
		}
		readers[r] = init

		b := make([]byte, 128)
		var onRead func(error, int, netip.AddrPort)
		onRead = func(err error, _ int, from netip.AddrPort) {
			if err == nil {
				entry := readers[r]
				entry.nRead++
				entry.from[from] = struct{}{}
				readers[r] = entry

				r.AsyncRead(b, onRead)
			}
		}
		r.AsyncRead(b, onRead)
	}

	w, err := NewUDPPeer(ioc, "udp", "")
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()

	var onWrite func(error, int)
	onWrite = func(err error, _ int) {
		if err == nil {
			w.AsyncWrite([]byte("hello"), multicastAddr, onWrite)
		}
	}
	w.AsyncWrite([]byte("hello"), multicastAddr, onWrite)

	start := time.Now()
	now := time.Now()
	for now.Sub(start).Seconds() < 5 {
		now = time.Now()
		ioc.PollOne()
	}

	log.Printf("done")

	for r, reader := range readers {
		log.Printf("reader index=%d n_read=%d from=%+v local_addr=%s",
			reader.index, reader.nRead, reader.from, r.LocalAddr())
		if reader.nRead == 0 {
			t.Fatal(
				"all readers should have read something since they all joined")
		}
	}
}

func TestUDPPeerIPv4_ReaderWriter(t *testing.T) {
	multicastIP := "224.0.0.18"
	multicastPort := 1234
	multicastAddr, err := netip.ParseAddrPort(
		fmt.Sprintf("%s:%d", multicastIP, multicastPort))
	if err != nil {
		t.Fatal(err)
	}

	ioc := sonic.MustIO()
	defer ioc.Close()

	r, err := NewUDPPeer(ioc, "udp", multicastAddr.String())
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()

	if err := r.Join(IP(multicastIP)); err != nil {
		t.Fatalf("reader could not join %s", multicastIP)
	} else {
		log.Printf("reader joined group %s", multicastIP)
	}

	receivedCount := make(map[uint64]int)
	var received []uint64

	rb := make([]byte, 8)
	var onRead func(error, int, netip.AddrPort)
	onRead = func(err error, _ int, _ netip.AddrPort) {
		if err == nil {
			seq := binary.BigEndian.Uint64(rb)
			receivedCount[seq]++
			received = append(received, seq)
			for i := 0; i < len(rb); i++ {
				rb[i] = 0
			}
			r.AsyncRead(rb, onRead)
		}
	}
	r.AsyncRead(rb, onRead)

	w, err := NewUDPPeer(ioc, "udp", "")
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()

	wb := make([]byte, 8)
	var seq uint64 = 1

	for i := 0; i < 100; i++ {
		time.Sleep(time.Millisecond)

		binary.BigEndian.PutUint64(wb, seq)
		seq++
		_, err = w.Write(wb, multicastAddr)
		if err != nil && err != sonicerrors.ErrWouldBlock {
			t.Fatalf("on the %d write err=%v", i, err)
		}

		for j := 0; j < 100; j++ {
			ioc.PollOne()
		}
	}

	for j := 0; j < 100; j++ {
		ioc.PollOne()
	}

	if len(received) == 0 || len(receivedCount) == 0 {
		t.Fatal("reader did not receive anything")
	}

	for num, count := range receivedCount {
		if count != 1 {
			if count == 0 {
				t.Fatalf("did not receive %d", num)
			} else {
				t.Fatalf("received duplicate of %d count=%d", num, count)
			}
		}
	}

	last := received[0]
	for _, recv := range received[1:] {
		if recv-last != 1 {
			fmt.Println(received)
			t.Fatal("did not receive in order")
		} else {
			last = recv
		}
	}
}

func TestUDPPeerIPv4_MultipleReadersSameBuffer(t *testing.T) {
	ioc := sonic.MustIO()
	defer ioc.Close()

	var (
		ips   = []string{"224.0.0.19", "224.0.0.20"}
		ports = []int{1234, 4321}
		addrs []netip.AddrPort
	)
	for i := 0; i < 2; i++ {
		addr, err := netip.ParseAddrPort(
			fmt.Sprintf("%s:%d", ips[i], ports[i]))
		if err != nil {
			t.Fatal(err)
		}
		addrs = append(addrs, addr)
	}

	var (
		chunk1, chunk2 [4]byte
		b              []byte
		parity         = 0
		readers        []*UDPPeer

		read []int
	)

	for i := 0; i < 2; i++ {
		r, err := NewUDPPeer(ioc, "udp", addrs[i].String())
		if err != nil {
			t.Fatal(err)
		}
		defer r.Close()

		if err := r.Join(IP(ips[i])); err != nil {
			t.Fatalf("reader could not join %s", ips[i])
		} else {
			log.Printf("reader joined group %s", ips[i])
		}

		id := i

		var fn func(error, int, netip.AddrPort)
		fn = func(err error, _ int, _ netip.AddrPort) {
			if err != nil {
				t.Fatal(err)
			}

			v := binary.BigEndian.Uint32(b)
			log.Printf(
				"reader %d read %d from %p",
				id,
				v,
				b,
			)
			read = append(read, int(v))

			parity++
			if parity%2 == 0 {
				b = chunk1[:]
			} else {
				b = chunk2[:]
			}

			for _, reader := range readers {
				reader.SetAsyncReadBuffer(b)
			}
			r.AsyncRead(b, fn)
		}
		b = chunk1[:]
		r.AsyncRead(b, fn)

		readers = append(readers, r)
	}

	var writers []*UDPPeer
	for i := 0; i < 2; i++ {
		w, err := NewUDPPeer(ioc, "udp", "")
		if err != nil {
			t.Fatal(err)
		}
		defer w.Close()

		writers = append(writers, w)
	}

	var wb [4]byte
	const Nops = 32

	for i := 0; i < Nops; i++ {
		time.Sleep(time.Millisecond)

		ix := i % 2
		binary.BigEndian.PutUint32(wb[:], uint32(i))

		_, err := writers[ix].Write(wb[:], addrs[ix])
		if err != nil && err != sonicerrors.ErrWouldBlock {
			t.Fatalf("on the %d write err=%v", i, err)
		}

		for j := 0; j < Nops; j++ {
			ioc.PollOne()
		}
	}
	for j := 0; j < Nops; j++ {
		ioc.PollOne()
	}

	// assert
	sort.Ints(read)

	if len(read) != Nops {
		t.Fatal("did not read correctly")
	}
	for i := 0; i < Nops; i++ {
		if read[i] != i {
			t.Fatal("did not read correctly")
		}
	}
}
