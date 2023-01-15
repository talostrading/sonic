package sonic

import (
	"errors"
	"fmt"
	"net"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/talostrading/sonic/internal"
	"github.com/talostrading/sonic/sonicerrors"
)

func sendTo(b []byte, addr string) error {
	fd, err := syscall.Socket(syscall.AF_INET, syscall.SOCK_DGRAM, 0)
	if err != nil {
		return err
	}
	defer func() {
		syscall.Close(fd)
	}()

	to, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return err
	}

	return syscall.Sendto(fd, b, 0, internal.ToSockaddr(to))
}

func recvFrom(b []byte, addr string) (int, net.Addr, error) {
	fd, err := syscall.Socket(syscall.AF_INET, syscall.SOCK_DGRAM, 0)
	if err != nil {
		return 0, nil, err
	}
	defer func() {
		syscall.Close(fd)
	}()

	localAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return 0, nil, err
	}

	err = syscall.Bind(fd, internal.ToSockaddr(localAddr))
	if err != nil {
		return 0, nil, err
	}

	n, peerAddr, err := syscall.Recvfrom(fd, b, 0)
	return n, internal.FromSockaddr(peerAddr), err
}

func TestPacketError(t *testing.T) {
	ioc := MustIO()
	defer ioc.Close()

	conn, err := NewPacketConn(ioc, "udp", "localhost:9080")
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	b := make([]byte, 1)
	_, _, err = conn.ReadFrom(b)
	if !errors.Is(err, sonicerrors.ErrWouldBlock) {
		t.Fatal("should return ErrWouldBlock")
	}

	addr, err := net.ResolveUDPAddr("udp", "localhost:8181")
	if err != nil {
		t.Fatal(err)
	}
	err = conn.WriteTo([]byte("hello"), addr)
	if err != nil {
		t.Fatal("should not return error")
	}
}

func TestPacketReadFrom(t *testing.T) {
	marker := make(chan struct{}, 1)
	go func() {
		<-marker
		for i := 0; i < 10; i++ {
			sendTo([]byte("hello"), "localhost:8080")
			time.Sleep(time.Millisecond)
		}
	}()

	ioc := MustIO()
	defer ioc.Close()

	conn, err := NewPacketConn(ioc, "udp", "localhost:8080")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		conn.Close()
		if !conn.Closed() {
			t.Fatal("connection should be closed")
		}
	}()

	if conn.LocalAddr() == nil {
		t.Fatal("PacketConn socket should be bound to a local address")
	}

	if strings.Split(conn.LocalAddr().String(), ":")[1] != "8080" {
		t.Fatalf("invalid port for local UDP address %v", conn.LocalAddr())
	}

	marker <- struct{}{}
	b := make([]byte, 128)
	for {
		b = b[:cap(b)]

		n, addr, err := conn.ReadFrom(b)
		if err != nil {
			if err == sonicerrors.ErrWouldBlock {
				continue
			} else {
				t.Fatal(err)
			}
		} else {
			b = b[:n]
			if string(b) != "hello" {
				t.Fatalf("wrong message expected \"hello\" but given \"%s\"", string(b))
			}
			if addr == nil {
				t.Fatal("address should not be empty")
			}
			break
		}
	}
}

func TestPacketAsyncReadFrom(t *testing.T) {
	marker := make(chan struct{}, 1)
	go func() {
		<-marker
		for i := 0; i < 100; i++ {
			sendTo([]byte("hello"), "localhost:8081")
			time.Sleep(time.Millisecond)
		}
	}()

	ioc := MustIO()
	defer ioc.Close()

	conn, err := NewPacketConn(ioc, "udp", "localhost:8081")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		conn.Close()
		if !conn.Closed() {
			t.Fatal("connection should be closed")
		}
	}()

	if conn.LocalAddr() == nil {
		t.Fatal("PacketConn socket should be bound to a local address")
	}

	if strings.Split(conn.LocalAddr().String(), ":")[1] != "8081" {
		t.Fatalf("invalid port for local UDP address %v", conn.LocalAddr())
	}

	nread := 0
	b := make([]byte, 128)
	var onRead AsyncReadCallbackPacket
	onRead = func(err error, n int, addr net.Addr) {
		nread++
		if err != nil {
			t.Fatal(err)
		} else {
			b = b[:n]
			if string(b) != "hello" {
				t.Fatalf("wrong message expected \"hello\" but given \"%s\"", string(b))
			}
			if addr == nil {
				t.Fatal("address should not be empty")
			}
			conn.AsyncReadFrom(b, onRead)
		}
	}
	conn.AsyncReadFrom(b, onRead)

	marker <- struct{}{}

	start := time.Now()
	for nread < 5 || time.Now().Sub(start) < time.Second {
		ioc.RunOneFor(time.Millisecond)
	}

	if nread == 0 {
		t.Fatal("did not read anything")
	}
}

func TestPacketWriteTo(t *testing.T) {
	marker := make(chan struct{}, 1)
	go func() {
		<-marker
		b := make([]byte, 128)
		n, addr, err := recvFrom(b, "localhost:8082")
		if err != nil {
			panic(err)
		}
		if addr == nil {
			panic("address should not be nil")
		}

		b = b[:n]
		if string(b) != "hello" {
			panic(fmt.Errorf("expected to receive hello but instead got %s", string(b)))
		}
		marker <- struct{}{}
	}()

	toAddr, err := net.ResolveUDPAddr("udp", "localhost:8082")
	if err != nil {
		t.Fatal(err)
	}

	ioc := MustIO()
	defer ioc.Close()

	conn, err := NewPacketConn(ioc, "udp", "" /* assign randomly */)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		conn.Close()
		if !conn.Closed() {
			t.Fatal("connection should be closed")
		}
	}()

	marker <- struct{}{}
outer:
	for {
		select {
		case <-marker:
			break outer
		default:
			time.Sleep(time.Millisecond)
			err = conn.WriteTo([]byte("hello"), toAddr)
			if err != nil {
				t.Fatal(err)
			}
		}
	}
}

func TestPacketAsyncWriteTo(t *testing.T) {
	marker := make(chan struct{}, 1)
	go func() {
		<-marker
		b := make([]byte, 128)
		n, addr, err := recvFrom(b, "localhost:8083")
		if err != nil {
			panic(err)
		}
		if addr == nil {
			panic("address should not be nil")
		}

		b = b[:n]
		if string(b) != "hello" {
			panic(fmt.Errorf("expected to receive hello but instead got %s", string(b)))
		}
		marker <- struct{}{}
	}()

	toAddr, err := net.ResolveUDPAddr("udp", "localhost:8083")
	if err != nil {
		t.Fatal(err)
	}

	ioc := MustIO()
	defer ioc.Close()

	conn, err := NewPacketConn(ioc, "udp", "" /* assign randomly */)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		conn.Close()
		if !conn.Closed() {
			t.Fatal("connection should be closed")
		}
	}()

	marker <- struct{}{}
	done := false
	conn.AsyncWriteTo([]byte("hello"), toAddr, func(err error) {
		done = true
		if err != nil {
			t.Fatal(err)
		}
	})

	now := time.Now()
	for !done || time.Now().Sub(now) < time.Second {
		ioc.RunOneFor(time.Millisecond)
	}
}
