package sonic

import (
	"fmt"
	"net"
	"testing"

	"github.com/talostrading/sonic/sonicopts"
)

func TestAsyncTCPClient(t *testing.T) {
	ioc := MustIO()
	defer ioc.Close()

	closer := make(chan struct{}, 1)

	go func() {
		ln, err := net.Listen("tcp", "localhost:8080")
		if err != nil {
			panic(err)
		}
		defer ln.Close()

		conn, err := ln.Accept()
		if err != nil {
			panic(err)
		}

		b := make([]byte, 128)
	outer:
		for {
			select {
			case <-closer:
				break outer
			default:
			}

			conn.Write([]byte("hello"))

			b = b[:cap(b)]
			n, err := conn.Read(b)
			if err != nil {
				panic(err)
			}

			if string(b[:n]) != "hello" {
				panic(fmt.Errorf("did not read %v", string(b)))
			}
		}
	}()

	conn, err := Dial(ioc, "tcp", "localhost:8080")
	if err != nil {
		panic(err)
	}

	b := make([]byte, 5)
	var onAsyncRead AsyncCallback
	onAsyncRead = func(err error, n int) {
		if err != nil {
			panic(err)
		}
		b = b[:n]
		if string(b) != "hello" {
			t.Fatalf("did not read %v", string(b))
		}

		conn.AsyncWriteAll(b, func(err error, n int) {
			if err != nil {
				panic(err)
			}

			b = b[:5]
			conn.AsyncReadAll(b, onAsyncRead)
		})
	}

	conn.AsyncReadAll(b, onAsyncRead)

	for i := 0; i < 1000; i++ {
		ioc.RunOne()
	}

	closer <- struct{}{}
}

func TestAsyncTCPListener(t *testing.T) {
	ioc := MustIO()
	defer ioc.Close()

	go func() {
		conn, err := net.Dial("tcp", "localhost:8081")
		if err != nil {
			panic(err)
		}

		b := make([]byte, 128)

		for {
			conn.Write([]byte("hello"))
			n, err := conn.Read(b)
			if err != nil {
				panic(err)
			}

			b = b[:n]

			if string(b) != "hello" {
				panic(fmt.Errorf("did not read %v", string(b)))
			}
		}
	}()

	ln, err := Listen(ioc, "tcp", "localhost:8081", sonicopts.Nonblocking(true))
	if err != nil {
		t.Fatal(err)
	}

	handle := func(conn Conn) {
		b := make([]byte, 5)
		var onAsyncRead AsyncCallback
		onAsyncRead = func(err error, n int) {
			if err != nil {
				t.Fatal(err)
			}

			b = b[:n]

			if string(b) != "hello" {
				t.Fatalf("did not read %v", string(b))
			}

			conn.AsyncWriteAll(b, func(err error, n int) {
				if err != nil {
					t.Fatal(err)
				}

				b = b[:cap(b)]
				conn.AsyncReadAll(b, onAsyncRead)
			})
		}
		conn.AsyncReadAll(b, onAsyncRead)
	}

	ln.AsyncAccept(func(err error, conn Conn) {
		if err != nil {
			t.Fatal(err)
		}

		handle(conn)
	})

	for i := 0; i < 1000; i++ {
		ioc.RunOne()
	}

	ln.Close()
}
