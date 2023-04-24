//go:build linux

package multicast

import (
	"fmt"
	"log"
	"net/netip"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TODO really don't know how to make this run on my mac
func TestUDPPeerIPv4_JoinSourceAndRead1(t *testing.T) {
	iffs, err := interfacesWithIP(4)
	if err == ErrNoInterfaces {
		log.Printf("skipping this test as not interfaces are available")
		return
	}

	r := newTestRW(t, "udp", "224.0.1.0:0")
	w := newTestRW(t, "udp", fmt.Sprintf("%s:0", iffs[0].ip))

	if w.peer.LocalAddr().IP.String() != iffs[0].ip.String() {
		t.Fatal("something wrong with binding to an interface address")
	}

	w.peer.LocalAddr()
	if err := r.peer.JoinSource("224.0.1.0", SourceIP(iffs[0].ip.String())); err != nil {
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
			_ = w.WriteNext(multicastAddr)
			time.Sleep(time.Millisecond)
		}
	}()
	wg.Wait()

	if atomic.LoadInt32(&count) == 0 {
		t.Fatal("reader did not read anything")
	}
}
