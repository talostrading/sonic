package multicast

import (
	"encoding/binary"
	"fmt"
	"github.com/talostrading/sonic"
	"log"
	"net"
	"net/netip"
	"os"
	"sync/atomic"
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

	ret := m.Run()
	if len(testInterfacesIPv4) == 0 && len(testInterfacesIPv6) == 0 {
		ret = 1
	}
	os.Exit(ret)
}

// To test:
// 1. Binding reader 0.0.0.0:0 and having two writers write to two different groups on the reader's port.
//    The reader should get packets from both.
// 2. Binding reader to 224.0.1.0:0 and having two writers writer to two different groups on the reader's port, one of
//    which should be 224.0.1.0. The reader should only get packets from one writer.
// 3. Binding two reader to 224.0.1.0:5001 and having a writer send packets to 224.0.1.0. Both readers should get those.
// 4. Binding readers to 224.0.1.0:5001 and having writers write to the address but a different port. Readers
//    should not get anything.
// 5. Binding to an explicit interface.
// 6. Two writers publishing to the same group, reader joining with Join, gets both, with JoinSource, gets only one,
//    with Join gets two then Block source gets one then Unblock source gets two
// 7. Reader JoinSourceGroup and LeaveSourceGroup
// 8. Reader source group.

type testRW struct {
	t        *testing.T
	ioc      *sonic.IO
	peer     *UDPPeer
	seq      uint64
	b        []byte
	closed   int32
	received map[netip.AddrPort]struct{}
}

func newTestRW(t *testing.T, network, addr string) *testRW {
	ioc := sonic.MustIO()
	peer, err := NewUDPPeer(ioc, network, addr)
	if err != nil {
		t.Fatal(err)
	}
	w := &testRW{
		t:        t,
		ioc:      ioc,
		peer:     peer,
		seq:      1,
		b:        make([]byte, 8),
		received: make(map[netip.AddrPort]struct{}),
	}
	return w
}

func (rw *testRW) WriteNext(multicastAddr string) error {
	addr, err := netip.ParseAddrPort(multicastAddr)
	if err != nil {
		rw.t.Fatal(err)
	}
	binary.BigEndian.PutUint64(rw.b, rw.seq)
	rw.seq++
	_, err = rw.peer.Write(rw.b, addr)
	return err
}

func (rw *testRW) ReadLoop(fn func(error, uint64, netip.AddrPort)) {
	var onRead func(error, int, netip.AddrPort)
	onRead = func(err error, n int, from netip.AddrPort) {
		if err == nil {
			rw.received[from] = struct{}{}
			fn(err, binary.BigEndian.Uint64(rw.b), from)
		}

		if atomic.LoadInt32(&rw.closed) == 0 {
			rw.peer.AsyncRead(rw.b, onRead)
		}
	}
	rw.peer.AsyncRead(rw.b, onRead)

	rw.ioc.Run()
}

func (rw *testRW) Close() {
	if atomic.LoadInt32(&rw.closed) == 0 {
		atomic.StoreInt32(&rw.closed, 1)
		rw.peer.Close()
		rw.ioc.Close()
	}
}

func (rw *testRW) ReceivedFrom() (xs []netip.AddrPort) {
	for addr := range rw.received {
		xs = append(xs, addr)
	}
	return xs
}
