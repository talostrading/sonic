package multicast

import (
	"encoding/binary"
	"errors"
	"fmt"
	"log"
	"net"
	"net/netip"
	"os"
	"sync/atomic"
	"testing"

	"github.com/talostrading/sonic"
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
	onRead = func(err error, _ int, from netip.AddrPort) {
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

var ErrNoInterfaces = errors.New("no interfaces available")

type interfaceWithIP struct {
	iff net.Interface
	ip  netip.Addr
}

func interfacesWithIP(v int) (ret []interfaceWithIP, err error) {
	iffs, err := net.Interfaces()
	if err != nil {
		return nil, err
	}

	if len(iffs) == 0 {
		return nil, ErrNoInterfaces
	}

	for _, iff := range iffs {
		if iff.Flags&net.FlagUp != net.FlagUp {
			continue
		}
		if iff.Flags&net.FlagMulticast != net.FlagMulticast {
			continue
		}

		addrs, err := iff.Addrs()
		if err != nil {
			continue
		}

		for _, a := range addrs {
			ip, _, err := net.ParseCIDR(a.String())
			if err != nil {
				continue
			}
			if ip.IsLoopback() {
				continue
			}
			addr, err := netip.ParseAddr(ip.String())
			if err != nil {
				continue
			}
			if !addr.IsValid() {
				continue
			}

			if v == 4 {
				if addr.Is4() || addr.Is4In6() {
					ret = append(ret, interfaceWithIP{
						iff: iff,
						ip:  addr,
					})
				}
			} else {
				panic("not supported")
			}
		}
	}
	return ret, nil
}
