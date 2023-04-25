//go:build darwin || netbsd || freebsd || openbsd || dragonfly

package multicast

import (
	"fmt"
	"github.com/talostrading/sonic"
	"log"
	"net/netip"
	"testing"
	"time"
)

// See TestUDPPeerIPv4_MultipleReadersOnINADDRANY_OneJoins1 in peer_ipv4_linux_test.go
func TestUDPPeerIPv4_MultipleReadersOnINADDRANY_OneJoins(t *testing.T) {
	ioc := sonic.MustIO()
	defer ioc.Close()

	multicastIP := "224.0.1.42"
	multicastPort := 1234
	multicastAddr, err := netip.ParseAddrPort(fmt.Sprintf("%s:%d", multicastIP, multicastPort))
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
		onRead = func(err error, n int, from netip.AddrPort) {
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
	onWrite = func(err error, n int) {
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
		log.Printf("reader index=%d n_read=%d from=%+v", reader.index, reader.nRead, reader.from)
		if reader.index != 0 /* the one that joined */ && reader.nRead != 0 {
			t.Fatal("only one the reader who joined should have read something")
		}
	}
}
