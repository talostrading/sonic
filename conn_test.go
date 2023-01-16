package sonic

import (
	"errors"
	"fmt"
	"io"
	"net"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/talostrading/sonic/sonicopts"
)

func TestConnUDPAsyncWrite(t *testing.T) {
	var nread uint32 = 0
	marker := make(chan struct{}, 1)
	go func() {
		udpAddr, err := net.ResolveUDPAddr("udp", "localhost:8084")
		if err != nil {
			panic(err)
		}
		udp, err := net.ListenUDP("udp", udpAddr)
		if err != nil {
			panic(err)
		}
		defer udp.Close()

		marker <- struct{}{}
		<-marker

		b := make([]byte, 128)
		for i := 0; i < 1000; i++ {
			n, err := udp.Read(b)
			if err == nil {
				b = b[:n]
				if string(b) != "hello" {
					panic("invalid message")
				}
				atomic.AddUint32(&nread, 1)
			}
		}

		marker <- struct{}{} // done reading
	}()
	<-marker

	ioc := MustIO()
	defer ioc.Close()

	conn, err := Dial(ioc, "udp", "localhost:8084")
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	var onWrite AsyncCallback
	onWrite = func(err error, _ int) {
		if err != nil {
			if err != io.EOF {
				t.Fatal(err)
			}
		} else {
			select {
			case <-marker:
			default:
				conn.AsyncWriteAll([]byte("hello"), onWrite)
			}
		}
	}

	marker <- struct{}{} // server can start
	conn.AsyncWriteAll([]byte("hello"), onWrite)

	ioc.RunPending()
	if atomic.LoadUint32(&nread) == 0 {
		t.Fatal("did not read anything")
	}
}

func TestConnUDPAsyncRead(t *testing.T) {
	marker := make(chan struct{}, 1)
	go func() {
		udpAddr, err := net.ResolveUDPAddr("udp", "localhost:8085")
		if err != nil {
			panic(err)
		}
		udp, err := net.DialUDP("udp", nil, udpAddr)
		if err != nil {
			panic(err)
		}
		defer udp.Close()

		<-marker

		for i := 0; i < 100; i++ {
			udp.Write([]byte("hello"))
			time.Sleep(time.Millisecond)
		}

		marker <- struct{}{}
	}()

	ioc := MustIO()
	defer ioc.Close()

	conn, err := ListenPacket(ioc, "udp", "localhost:8085", sonicopts.ReuseAddr(true))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	nread := 0
	b := make([]byte, 128)
	var onRead AsyncReadCallbackPacket
	onRead = func(err error, n int, fromAddr net.Addr) {
		if err != nil {
			if err != io.EOF {
				t.Fatal(err)
			}
		} else {
			b = b[:n]
			if string(b) != "hello" {
				t.Fatal("wrong message")
			}

			nread++

			select {
			case <-marker:
			default:
				b = b[:cap(b)]
				conn.AsyncReadFrom(b, onRead)
			}
		}
	}

	conn.AsyncReadFrom(b, onRead)
	marker <- struct{}{}

	now := time.Now()
	for nread < 5 || time.Now().Sub(now) < time.Second {
		ioc.PollOne()
	}
	if nread == 0 {
		t.Fatal("did not read anything")
	}
}

func TestConnTCPAsyncEchoClient(t *testing.T) {
	marker := make(chan struct{}, 1)

	go func() {
		ln, err := net.Listen("tcp", "localhost:8086")
		if err != nil {
			panic(err)
		}
		defer ln.Close()

		marker <- struct{}{}

		conn, err := ln.Accept()
		if err != nil {
			panic(err)
		}
		defer conn.Close()

		b := make([]byte, 128)
	outer:
		for {
			select {
			case <-marker:
				break outer
			default:
			}

			conn.Write([]byte("hello"))

			b = b[:cap(b)]
			n, err := conn.Read(b)
			if err != nil {
				break outer
			}

			if string(b[:n]) != "hello" {
				panic(fmt.Errorf("did not read %v", string(b)))
			}

			time.Sleep(time.Millisecond)
		}
	}()
	<-marker

	ioc := MustIO()
	defer ioc.Close()

	conn, err := Dial(ioc, "tcp", "localhost:8086")
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	b := make([]byte, 5)
	var onAsyncRead AsyncCallback
	onAsyncRead = func(err error, n int) {
		if err != nil {
			if err != io.EOF {
				t.Fatal(err)
			}
		} else {
			b = b[:n]
			if string(b) != "hello" {
				t.Fatalf("did not read %v", string(b))
			}

			conn.AsyncWriteAll(b, func(err error, n int) {
				if err != nil {
					if !errors.Is(err, io.EOF) || !errors.Is(err, syscall.EPIPE) {
						t.Fatal(err)
					}
				} else {
					b = b[:5]
					conn.AsyncReadAll(b, onAsyncRead)
				}
			})
		}
	}

	conn.AsyncReadAll(b, onAsyncRead)

	for i := 0; i < 10; i++ {
		ioc.RunOne()
	}

	marker <- struct{}{}
}

func TestConnTCPReadHandlesError(t *testing.T) {
	marker := make(chan struct{}, 1)
	go func() {
		ln, err := net.Listen("tcp", "localhost:8087")
		if err != nil {
			panic(err)
		}

		marker <- struct{}{}

		conn, err := ln.Accept()
		if err != nil {
			panic(err)
		}
		defer conn.Close()

		_, err = conn.Write([]byte("hello"))
		if err != nil {
			panic(err)
		}

	}()
	<-marker

	ioc := MustIO()
	defer ioc.Close()

	conn, err := Dial(ioc, "tcp", "localhost:8087")
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	done := false
	b := make([]byte, 128)
	var onAsyncRead AsyncCallback
	onAsyncRead = func(err error, n int) {
		if err != nil {
			if err != io.EOF {
				t.Fatal(err)
			} else {
				done = true
			}
		} else {
			b = b[:cap(b)]
			conn.AsyncReadAll(b, onAsyncRead)
		}
	}
	conn.AsyncReadAll(b, onAsyncRead)

	ioc.RunPending()

	if !done {
		t.Fatal("test did not run to completion")
	}
}

func TestConnTCPWriteHandlesError(t *testing.T) {
	marker := make(chan struct{}, 1)

	go func() {
		ln, err := net.Listen("tcp", "localhost:8088")
		if err != nil {
			panic(err)
		}
		defer ln.Close()

		marker <- struct{}{}

		conn, err := ln.Accept()
		if err != nil {
			panic(err)
		}
		defer conn.Close()

		b := make([]byte, 128)
		_, err = conn.Read(b)
		if err != nil {
			panic(err)
		}
	}()
	<-marker

	ioc := MustIO()
	defer ioc.Close()

	conn, err := Dial(ioc, "tcp", "localhost:8088")
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	done := false
	var onAsyncWrite AsyncCallback
	onAsyncWrite = func(err error, n int) {
		if err != nil {
			if errors.Is(err, syscall.EPIPE) || errors.Is(err, syscall.ECONNRESET) {
				done = true
			} else {
				t.Fatal(err)
			}
		} else {
			conn.AsyncWriteAll([]byte("hello"), onAsyncWrite)
		}
	}
	conn.AsyncWriteAll([]byte("hello"), onAsyncWrite)

	ioc.RunPending()

	if !done {
		t.Fatal("test did not run to completion")
	}
}

func TestUDPAddresses(t *testing.T) {
	marker := make(chan struct{}, 1)
	defer close(marker)
	go func() {
		udpAddr, err := net.ResolveUDPAddr("udp", "localhost:8100")
		if err != nil {
			panic(err)
		}

		udp, err := net.ListenUDP("udp", udpAddr) // this never blocks
		if err != nil {
			panic(err)
		}
		defer udp.Close()
		marker <- struct{}{}
		<-marker
	}()
	<-marker

	ioc := MustIO()
	defer ioc.Close()

	udp, err := Dial(ioc, "udp", "localhost:8100")
	if err != nil {
		t.Fatal(err)
	}
	defer udp.Close()

	if addr := udp.LocalAddr().(*net.UDPAddr); addr.Port == 0 {
		t.Fatalf("invalid local address %s", addr)
	}
	if addr := udp.RemoteAddr().(*net.UDPAddr); addr.Port != 8100 {
		t.Fatalf("invalid remote address %s", addr)
	}
	marker <- struct{}{}
}

func TestTCPAddresses(t *testing.T) {
	marker := make(chan struct{}, 1)
	defer close(marker)
	go func() {
		tcp, err := net.Listen("tcp", "localhost:8101") // this never blocks
		if err != nil {
			panic(err)
		}
		defer tcp.Close()

		marker <- struct{}{}

		conn, err := tcp.Accept()
		if err != nil {
			panic(err)
		}
		defer conn.Close()

		<-marker
	}()
	<-marker

	ioc := MustIO()
	defer ioc.Close()

	tcp, err := Dial(ioc, "tcp", "localhost:8101")
	if err != nil {
		t.Fatal(err)
	}
	defer tcp.Close()

	if addr := tcp.LocalAddr().(*net.TCPAddr); addr.Port == 0 {
		t.Fatalf("invalid local address %s", addr)
	}
	if addr := tcp.RemoteAddr().(*net.TCPAddr); addr.Port != 8101 {
		t.Fatalf("invalid remote address %s", addr)
	}
	marker <- struct{}{}
}
