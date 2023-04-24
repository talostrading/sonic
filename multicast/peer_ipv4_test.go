package multicast

import (
	"fmt"
	"github.com/talostrading/sonic"
	"github.com/talostrading/sonic/net/ipv4"
	"log"
	"net"
	"net/netip"
	"sync"
	"sync/atomic"
	"testing"
	"time"
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

	ip, err := netip.ParseAddr("224.0.1.0")
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

	if err := peer.Join("224.0.1.0"); err != nil {
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

	if err := peer.JoinOn("224.0.1.0", InterfaceName(testInterfacesIPv4[0].Name)); err != nil {
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

	if err := peer.JoinSource("224.0.1.0", SourceIP(iffs[0].ip.String())); err != nil {
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
		"224.0.1.0",
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
			fmt.Printf("%s GetMulticastInterface_Inet4Addr addr=%s\n", iff.Name, addr.String())
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
			fmt.Printf("%s GetMulticastInterface_Index interface_index=%d\n", iff.Name, interfaceIndex)
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
			fmt.Printf("%s GetMulticastInterface_Inet4Addr addr=%s\n", iff.Name, addr.String())
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
			fmt.Printf("%s GetMulticastInterface_Index interface_index=%d\n", iff.Name, interfaceIndex)
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
	// 1 reader on INADDR_ANY joining 224.0.1.0, 1 writer on 224.0.1.0:<reader_port>

	r := newTestRW(t, "udp", "")
	defer r.Close()

	multicastIP := "224.0.1.0"
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
		r.ReadLoop(func(err error, seq uint64, from netip.AddrPort) {
			if err != nil {
				t.Fatal(err)
			} else {
				count++
				if count == 10 || time.Now().Sub(start).Seconds() > 1 /* just to not have it hang */ {
					r.Close()
				}
			}
		})
	}()

	go func() {
		defer wg.Done()
		for i := 0; i < 10; i++ {
			if err := w.WriteNext(multicastAddr); err != nil {
				t.Fatal(err)
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
	// 2 writers on multicastAddr: 224.0.1.0:<reader_port>.
	// reader joins 224.0.1.0. Reader should get from both.

	r := newTestRW(t, "udp", "")
	defer r.Close()

	multicastIP := "224.0.1.0"
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
		r.ReadLoop(func(err error, seq uint64, from netip.AddrPort) {
			if err != nil {
				t.Fatal(err)
			} else {
				count[from]++
				stopCount := 0
				for _, c := range count {
					if c == 10 {
						stopCount++
					}
				}

				if stopCount == 2 || time.Now().Sub(start).Seconds() > 1 /* just to not have it hang */ {
					r.Close()
				}
			}
		})
	}()

	go func() {
		defer wg.Done()
		for i := 0; i < 10; i++ {
			if err := w1.WriteNext(multicastAddr); err != nil {
				t.Fatal(err)
			}
			time.Sleep(time.Millisecond)
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 10; i++ {
			if err := w2.WriteNext(multicastAddr); err != nil {
				t.Fatal(err)
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
	// - one on 224.0.1.0:<reader_port>
	// - two on 224.0.2.0:<reader_port>
	// The reader only joins 224.0.1.0. Should only get from writer 1.

	r := newTestRW(t, "udp", "")
	defer r.Close()

	multicastIP1 := "224.0.1.0"
	multicastAddr1 := fmt.Sprintf("%s:%d", multicastIP1, r.peer.LocalAddr().Port)
	if err := r.peer.Join(IP(multicastIP1)); err != nil {
		t.Fatal(err)
	}

	multicastIP2 := "224.0.2.0"
	multicastAddr2 := fmt.Sprintf("%s:%d", multicastIP2, r.peer.LocalAddr().Port)

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
		r.ReadLoop(func(err error, seq uint64, from netip.AddrPort) {
			if err != nil {
				t.Fatal(err)
			} else {
				count[from]++
				stopCount := 0
				for _, c := range count {
					if c == 10 {
						stopCount++
					}
				}

				if stopCount == 1 || time.Now().Sub(start).Seconds() > 1 /* just to not have it hang */ {
					r.Close()
				}
			}
		})
	}()

	go func() {
		defer wg.Done()
		for i := 0; i < 10; i++ {
			if err := w1.WriteNext(multicastAddr1); err != nil {
				t.Fatal(err)
			}
			time.Sleep(time.Millisecond)
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 10; i++ {
			if err := w2.WriteNext(multicastAddr2); err != nil {
				t.Fatal(err)
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
	// - one on 224.0.1.0:<reader_port>
	// - two on 224.0.1.0:<not_reader_port>
	// The reader only joins 224.0.1.0. Should only get from writer 1 since writer 2 does not publish on the reader's
	// port.

	r := newTestRW(t, "udp", "")
	defer r.Close()

	multicastIP1 := "224.0.1.0"
	multicastAddr1 := fmt.Sprintf("%s:%d", multicastIP1, r.peer.LocalAddr().Port)
	if err := r.peer.Join(IP(multicastIP1)); err != nil {
		t.Fatal(err)
	}

	multicastAddr2 := fmt.Sprintf("%s:%d", multicastIP1, r.peer.LocalAddr().Port+1)

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
		r.ReadLoop(func(err error, seq uint64, from netip.AddrPort) {
			if err != nil {
				t.Fatal(err)
			} else {
				count[from]++
				stopCount := 0
				for _, c := range count {
					if c == 10 {
						stopCount++
					}
				}

				if stopCount == 1 || time.Now().Sub(start).Seconds() > 1 /* just to not have it hang */ {
					r.Close()
				}
			}
		})
	}()

	go func() {
		defer wg.Done()
		for i := 0; i < 10; i++ {
			if err := w1.WriteNext(multicastAddr1); err != nil {
				t.Fatal(err)
			}
			time.Sleep(time.Millisecond)
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 10; i++ {
			if err := w2.WriteNext(multicastAddr2); err != nil {
				t.Fatal(err)
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
	// 1 reader bound to 224.0.1.0:0(so random port). Joins nothing.
	// 1 writer on 224.0.1.0:<reader_port>.
	// Reader should get nothing.

	multicastIP := "224.0.1.0"

	r := newTestRW(t, "udp", fmt.Sprintf("%s:0", multicastIP))
	defer r.Close()

	multicastAddr := fmt.Sprintf("%s:%d", multicastIP, r.peer.LocalAddr().Port)

	w := newTestRW(t, "udp", "")
	defer w.Close()

	readerGot := 0
	go func() {
		r.ReadLoop(func(err error, seq uint64, from netip.AddrPort) {
			if err != nil {
				t.Fatal(err)
			} else {
				readerGot += 1
			}
		})
	}()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 10; i++ {
			if err := w.WriteNext(multicastAddr); err != nil {
				t.Fatal(err)
			}
			time.Sleep(time.Millisecond)
		}
	}()
	wg.Wait()

	r.ioc.Close()

	if len(r.ReceivedFrom()) != 0 {
		t.Fatal("should have received from none")
	}

	fmt.Println(r.received)
}

func TestUDPPeerIPv4_Reader6(t *testing.T) {
	// Counterpart to TestReader2.
	//
	// 1 reader on 224.0.3.0:0(so random port). Joins both 224.0.3.0 and 224.0.4.0.
	// 2 writers:
	// - one on 224.0.3.0:<reader_port>
	// - two on 224.0.4.0:<not_reader_port>
	//
	// The reader joins both groups, but it's bound to 224.0.3.0, which has a filtering role, meaning that it should
	// only get from 224.0.3.0 and not also from 224.0.4.0.

	r := newTestRW(t, "udp", "224.0.3.0:0")
	defer r.Close()

	multicastIP1 := "224.0.3.0"
	multicastAddr1 := fmt.Sprintf("%s:%d", multicastIP1, r.peer.LocalAddr().Port)
	if err := r.peer.Join(IP(multicastIP1)); err != nil {
		t.Fatal(err)
	}

	multicastIP2 := "224.0.4.0"
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
		r.ReadLoop(func(err error, seq uint64, from netip.AddrPort) {
			if err != nil {
				t.Fatal(err)
			} else {
				count[from]++
				stopCount := 0
				for _, c := range count {
					if c == 10 {
						stopCount++
					}
				}

				if stopCount == 1 || time.Now().Sub(start).Seconds() > 1 /* just to not have it hang */ {
					r.Close()
				}
			}
		})
	}()

	go func() {
		defer wg.Done()
		for i := 0; i < 10; i++ {
			if err := w1.WriteNext(multicastAddr1); err != nil {
				t.Fatal(err)
			}
			time.Sleep(time.Millisecond)
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 10; i++ {
			if err := w2.WriteNext(multicastAddr2); err != nil {
				t.Fatal(err)
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

func TestUDPPeerIPv4_Reader7(t *testing.T) {
	// 1 reader on localhost:0(so random port). Joins 224.0.1.0.
	// 1 writer bound to 0.0.0.0:0 and sending on 224.0.1.0:<reader_port>.
	// Reader should not receive anything.
	// TODO still not entirely sure what happens when binding the writer to a unicast address.

	r := newTestRW(t, "udp", "localhost:0")
	defer r.Close()

	multicastIP := "224.0.1.0"
	multicastAddr := fmt.Sprintf("%s:%d", multicastIP, r.peer.LocalAddr().Port)
	if err := r.peer.Join(IP(multicastIP)); err != nil {
		t.Fatal(err)
	}

	w := newTestRW(t, "udp", "")
	defer w.Close()

	readerGot := 0
	go func() {
		r.ReadLoop(func(err error, seq uint64, from netip.AddrPort) {
			if err != nil {
				t.Fatal(err)
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
			if err := w.WriteNext(multicastAddr); err != nil {
				t.Fatal(err)
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
		log.Printf("not running this one as we don't have enough multicast interfaces")
		for _, iff := range testInterfacesIPv4 {
			log.Printf("interface name=%s", iff.Name)
		}
		return
	}

	readerInterface := testInterfacesIPv4[0]
	writerInterface := testInterfacesIPv4[0]
	log.Printf("reader_interface=%s writer_interface=%s", readerInterface.Name, writerInterface.Name)

	r := newTestRW(t, "udp", "")
	defer r.Close()
	if err := r.peer.SetInbound(readerInterface.Name); err != nil {
		t.Fatal(err)
	}
	if err := r.peer.Join("224.0.1.0"); err != nil {
		t.Fatal(err)
	}

	w := newTestRW(t, "udp", "")
	defer w.Close()
	if err := r.peer.SetOutboundIPv4(writerInterface.Name); err != nil {
		t.Fatal(err)
	}

	multicastAddr := fmt.Sprintf("224.0.1.0:%d", r.peer.LocalAddr().Port)

	var count int32 = 0
	go func() {
		r.ReadLoop(func(err error, seq uint64, from netip.AddrPort) {
			atomic.AddInt32(&count, 1)
		})
	}()

	time.Sleep(time.Millisecond)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 50; i++ {
			if err := w.WriteNext(multicastAddr); err != nil {
				t.Fatal(err)
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
	// TODO warning - this test is not correct. We explicitly tell the kernel to not loopback multicast packets.
	// SetInbound might not be correct but I need a proper setup to test that.

	if len(testInterfacesIPv4) < 2 {
		log.Printf("not running this one as we don't have enough multicast interfaces")
		for _, iff := range testInterfacesIPv4 {
			log.Printf("interface name=%s", iff.Name)
		}
		return
	}

	readerInterface := testInterfacesIPv4[0]
	writerInterface := testInterfacesIPv4[1]
	log.Printf("reader_interface=%s writer_interface=%s", readerInterface.Name, writerInterface.Name)

	r := newTestRW(t, "udp", "")
	defer r.Close()
	if err := r.peer.SetInbound(readerInterface.Name); err != nil {
		t.Fatal(err)
	}
	if err := r.peer.Join("224.0.1.0"); err != nil {
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

	multicastAddr := fmt.Sprintf("224.0.1.0:%d", r.peer.LocalAddr().Port)

	var count int32 = 0
	go func() {
		r.ReadLoop(func(err error, seq uint64, from netip.AddrPort) {
			atomic.AddInt32(&count, 1)
		})
	}()

	time.Sleep(time.Millisecond)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 50; i++ {
			if err := w.WriteNext(multicastAddr); err != nil {
				t.Fatal(err)
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
	r := newTestRW(t, "udp", "224.0.1.0:0")
	w := newTestRW(t, "udp", "")

	w.peer.LocalAddr()
	if err := r.peer.Join("224.0.1.0"); err != nil {
		t.Fatal(err)
	}

	var count int32
	go func() {
		r.ReadLoop(func(err error, _ uint64, from netip.AddrPort) {
			if err == nil {
				atomic.AddInt32(&count, 1)
			}
		})
	}()

	multicastAddr := fmt.Sprintf("224.0.1.0:%d", r.peer.LocalAddr().Port)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()

		for i := 0; i < 10; i++ {
			if err := w.WriteNext(multicastAddr); err != nil {
				t.Fatal(err)
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

	r := newTestRW(t, "udp", "224.0.1.0:0")
	w := newTestRW(t, "udp", "")

	w.peer.LocalAddr()
	if err := r.peer.JoinOn("224.0.1.0", InterfaceName(iffs[0].iff.Name)); err != nil {
		t.Fatal(err)
	}

	var count int32
	go func() {
		r.ReadLoop(func(err error, _ uint64, from netip.AddrPort) {
			if err == nil {
				atomic.AddInt32(&count, 1)
			}
		})
	}()

	multicastAddr := fmt.Sprintf("224.0.1.0:%d", r.peer.LocalAddr().Port)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()

		for i := 0; i < 10; i++ {
			if err := w.WriteNext(multicastAddr); err != nil {
				t.Fatal(err)
			}
			time.Sleep(time.Millisecond)
		}
	}()
	wg.Wait()

	if atomic.LoadInt32(&count) == 0 {
		t.Fatal("reader did not read anything")
	}
}
