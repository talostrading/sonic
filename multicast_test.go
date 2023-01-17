package sonic

import (
	"github.com/talostrading/sonic/internal"
	"github.com/talostrading/sonic/sonicopts"
	"net"
	"os"
	"sync"
	"syscall"
	"testing"
	"time"
)

var multicastTestInterface *net.Interface

func TestMain(t *testing.M) {
	// TODO maybe loopback such that these also work on circleci
	var err error
	multicastTestInterface, err = net.InterfaceByName("en0")
	if err != nil {
		panic(err)
	}

	os.Exit(t.Run())
}

type testMulticast struct {
	fd            int
	multicastAddr *net.UDPAddr
	bindAddr      *net.UDPAddr
}

func newTestMulticast(multicastAddr *net.UDPAddr) (*testMulticast, error) {
	fd, err := syscall.Socket(syscall.AF_INET, syscall.SOCK_DGRAM, 0)
	if err != nil {
		return nil, err
	}

	if err := internal.ApplyOpts(fd, sonicopts.ReuseAddr(true)); err != nil {
		return nil, err
	}

	// this is just to generate the bind addr, so we can test source IP filters
	if err := syscall.Connect(fd, internal.ToSockaddr(multicastAddr)); err != nil {
		syscall.Close(fd)
		return nil, err
	}

	bindAddr, err := internal.SocketAddressUDP(fd)
	if err != nil {
		syscall.Close(fd)
		return nil, err
	}

	return &testMulticast{
		fd:            fd,
		multicastAddr: multicastAddr,
		bindAddr:      bindAddr,
	}, nil
}

func (s *testMulticast) Run(n int, msg []byte, pause time.Duration) error {
	for i := 0; i < n; i++ {
		if err := syscall.Sendto(s.fd, msg, 0, internal.ToSockaddr(s.multicastAddr)); err != nil {
			return err
		}
		time.Sleep(pause)
	}
	return nil
}

func (s *testMulticast) Close() {
	syscall.Close(s.fd)
}

func TestMulticastIPv4JoinNoFilter(t *testing.T) {
	multicastAddr, err := net.ResolveUDPAddr("udp4", "224.0.1.0:40000")
	if err != nil {
		t.Fatal(err)
	}

	marker := make(chan struct{}, 1)
	defer close(marker)
	go func() {
		srv, err := newTestMulticast(multicastAddr)
		if err != nil {
			panic(err)
		}
		defer srv.Close()

		<-marker
		srv.Run(100, []byte("hello"), time.Millisecond)
	}()

	ioc := MustIO()
	defer ioc.Close()

	client, err := NewMulticastClient(ioc, multicastTestInterface, net.IPv4zero)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	if err := client.Join(multicastAddr); err != nil {
		t.Fatal(err)
	}
	marker <- struct{}{}

	nread := 0
	b := make([]byte, 128)
	var onRead AsyncReadCallbackPacket
	onRead = func(err error, n int, addr net.Addr) {
		if err != nil {
			t.Fatal(err)
		} else {
			nread++
			b = b[:n]
			b = b[:cap(b)]
			client.AsyncReadFrom(b, onRead)
		}
	}
	client.AsyncReadFrom(b, onRead)

	start := time.Now()
	for nread < 50 && time.Now().Sub(start) < time.Second {
		ioc.PollOne()
	}
	if nread == 0 {
		t.Fatal("client did not read anything")
	}
}

func TestMulticastIPv4JoinWithFilter(t *testing.T) {
	// 2 servers sending on the group, we join both
	// TODO it is hard to test this locally because we need two different IP addresses, one for each server,
	// hence two interfaces

	multicastAddr, err := net.ResolveUDPAddr("udp4", "224.0.1.0:40000")
	if err != nil {
		t.Fatal(err)
	}

	marker1 := make(chan struct{}, 1)
	defer close(marker1)
	marker2 := make(chan struct{}, 1)
	defer close(marker2)

	// we have to setup servers in order such that we don't run into the case where both connect() calls
	// try to bind to the same port
	var setup sync.WaitGroup

	var srv1 *testMulticast
	setup.Add(1)
	go func() {
		srv1, err = newTestMulticast(multicastAddr)
		if err != nil {
			panic(err)
		}
		defer srv1.Close()
		setup.Done()

		<-marker1
		srv1.Run(1000, []byte("hello_srv1"), time.Millisecond)
	}()
	setup.Wait()

	var srv2 *testMulticast
	setup.Add(1)
	go func() {
		srv2, err = newTestMulticast(multicastAddr)
		if err != nil {
			panic(err)
		}
		defer srv2.Close()
		setup.Done()

		<-marker2
		srv2.Run(1000, []byte("hello_srv2"), time.Millisecond)
	}()
	setup.Wait()

	ioc := MustIO()
	defer ioc.Close()

	client, err := NewMulticastClient(ioc, multicastTestInterface, net.IPv4zero)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	if err := client.Join(multicastAddr, srv1.bindAddr, srv2.bindAddr); err != nil {
		t.Fatal(err)
	}
	marker1 <- struct{}{}
	marker2 <- struct{}{}

	nread1, nread2 := 0, 0
	b := make([]byte, 128)
	var onRead AsyncReadCallbackPacket
	onRead = func(err error, n int, addr net.Addr) {
		if err != nil {
			t.Fatal(err)
		} else {
			b = b[:n]
			if string(b) == "hello_srv1" {
				nread1++
			}
			if string(b) == "hello_srv2" {
				nread2++
			}
			b = b[:cap(b)]
			client.AsyncReadFrom(b, onRead)
		}
	}
	client.AsyncReadFrom(b, onRead)

	start := time.Now()
	for nread1 < 50 && nread2 < 50 && time.Now().Sub(start) < time.Second {
		ioc.PollOne()
	}
	if nread1 == 0 || nread2 == 0 {
		t.Fatal("client did not read anything")
	}
}
