//go:build linux

package multicast

import (
	"fmt"
	"github.com/talostrading/sonic"
	"github.com/talostrading/sonic/net/ipv4"
	"github.com/talostrading/sonic/sonicerrors"
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

// There is an equivalent test in peer_ipv4_bsd_test. The only difference between them is that on Linux, if one peer
// joins a group while bound to INADDR_ANY, all other peers bound to INADDR_ANY will also automatically join the same
// multicast group, even though they have not explicitly joined.
//
// Why? Because that's the default in Linux. It has to do with the flag IP_MULTICAST_ALL. That is 1 by default and the
// behaviour is as described above.
//
// What do we actually want? We want only one reader to receive datagrams - the one that joined. The rest of them should
// not receive anything. That is the default behaviour on BSD, so that's why we separate these files. Below this test
// you'll find the one where we set IP_MULTICAST_ALL to 0 which gives us the desired behaviour, like on BSD.
//
// Now IP_MULTICAST_ALL is not defined in syscall on linux/amd64. Grepping torvalds/kernel, I see that IP_MULTICAST_ALL
// is 49. See ipv4 on how we implement it. What we could've done is use CGO to include <netinet.h> which would've given
// us access to the defined IP_MULTICAST_ALL. However, that means dynamically linking to glibc, at least until 1.21,
// which is a not desirable as sonic binaries are run on boomer finance boxes, with old kernels (4.18) and old glibc.
// So we hope IP_MULTICAST_ALL stays 49 until go1.21 when the dynamic glibc link is not needed anymore.
//
// Note that we set IP_MULTICAST_ALL to 0 in the constructor of UDPPeer as that's what we consider correct behaviour:
// multicast should be opt-in, not opt-out.
func TestUDPPeerIPv4_MultipleReadersOnINADDRANY_OneJoins_IP_MULTICAST_ALL_is_1_default(t *testing.T) {
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

		// Set to false in the constructor of UDPPeer. By default, it is true. We test the default here.
		if err := ipv4.SetMulticastAll(r.socket, true); err != nil {
			t.Fatal(err)
		}

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
		if reader.nRead == 0 {
			t.Fatal("all readers should have read something")
		}
		log.Printf("reader index=%d n_read=%d from=%+v", reader.index, reader.nRead, reader.from)
	}
}

// TODO I don't know why this fails on BSD
func TestUDPPeerIPv4_JoinSourceAndRead(t *testing.T) {
	iffs, err := interfacesWithIP(4)
	if err == ErrNoInterfaces {
		log.Printf("skipping this test as not interfaces are available")
		return
	}

	r := newTestRW(t, "udp", "224.0.2.0:0")
	w := newTestRW(t, "udp", fmt.Sprintf("%s:0", iffs[0].ip))

	if w.peer.LocalAddr().IP.String() != iffs[0].ip.String() {
		t.Fatal("something wrong with binding to an interface address")
	}

	w.peer.LocalAddr()
	if err := r.peer.JoinSource("224.0.2.0", SourceIP(iffs[0].ip.String())); err != nil {
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

	multicastAddr := fmt.Sprintf("224.0.2.0:%d", r.peer.LocalAddr().Port)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()

		for i := 0; i < 10; i++ {
			if err := w.WriteNext(multicastAddr); err != nil && err != sonicerrors.ErrNoBufferSpaceAvailable {
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

// TODO I don't know why this fails on BSD
func TestUDPPeerIPv4_JoinReadBlockUnblockRead(t *testing.T) {
	ioc := sonic.MustIO()
	defer ioc.Close()

	multicastIP := "224.0.1.0"
	r, err := NewUDPPeer(ioc, "udp", fmt.Sprintf("%s:0", multicastIP))
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()

	log.Printf("reader local_addr=%s", r.LocalAddr())

	multicastAddr, err := netip.ParseAddrPort(fmt.Sprintf("%s:%d", multicastIP, r.LocalAddr().Port))
	if err != nil {
		t.Fatal(err)
	}

	if err := r.Join(IP(multicastIP)); err != nil {
		t.Fatal(err)
	}

	var (
		onRead func(error, int, netip.AddrPort)
		rb     = make([]byte, 128)

		first      = true
		writerAddr netip.AddrPort
		nRead      int
	)
	onRead = func(err error, n int, from netip.AddrPort) {
		if err == nil {
			if first {
				first = false
				writerAddr = from
			}
			nRead++
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
		nTotal  = 0
	)
	onWrite = func(err error, n int) {
		if err == nil {
			nTotal++
			w.AsyncWrite(wb, multicastAddr, onWrite)
		} else {
			log.Printf("err=%v", err)
		}
	}
	w.AsyncWrite(wb, multicastAddr, onWrite)

	log.Printf("writer local_addr=%s", w.LocalAddr())

	blockTimer, err := sonic.NewTimer(ioc)
	if err != nil {
		t.Fatal(err)
	}
	defer blockTimer.Close()

	err = blockTimer.ScheduleOnce(2*time.Second, func() {
		log.Printf("blocking source %s", writerAddr)
		if err := r.BlockSource(IP(multicastIP), SourceIP(writerAddr.Addr().String())); err != nil {
			t.Fatal(err)
		} else {
			log.Printf("blocked source %s n_read=%d", writerAddr, nRead)
			nRead = 0
			err = blockTimer.ScheduleOnce(2*time.Second, func() {
				log.Printf("unblocking source %s", writerAddr)
				if err := r.UnblockSource(IP(multicastIP), SourceIP(writerAddr.Addr().String())); err != nil {
					t.Fatal(err)
				} else {
					log.Printf("unblocked source %s n_read=%d", writerAddr, nRead)
					nRead = 0
				}
			})
			if err != nil {
				t.Fatal(err)
			}
		}
	})
	if err != nil {
		t.Fatal(err)
	}

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

	log.Printf("done n_read=%d after unblocking", nRead)

	if nRead <= 0 {
		t.Fatal("did not read anything after unblocking")
	}
}
