package sonic

import (
	"github.com/talostrading/sonic/internal"
	"net"
	"testing"
)

func TestSocket_ReadBlocking(t *testing.T) {
	flag := make(chan struct{}, 1)
	defer func() { <-flag }()

	go func() {
		ln, err := net.Listen("tcp", "localhost:8080")
		if err != nil {
			panic(err)
		}
		defer func() {
			ln.Close()
			flag <- struct{}{}
		}()
		flag <- struct{}{}

		conn, err := ln.Accept()
		if err != nil {
			panic(err)
		}

		_, err = conn.Write([]byte("hello"))
		if err != nil {
			panic(err)
		}
	}()

	ioc := MustIO()
	defer ioc.Close()

	<-flag

	socket, err := Connect(ioc, "tcp", "localhost:8080")
	if err != nil {
		t.Fatal(err)
	}

	b := make([]byte, 128)
	n, err := socket.Read(b)
	if err != nil {
		t.Fatal(err)
	}
	b = b[:n]

	if string(b) != "hello" {
		t.Fatal("invalid read")
	}

	ioc.PollOne()

	if v, err := internal.IsNonblocking(socket.RawFd()); err != nil || v {
		t.Fatal("socket is nonblocking")
	}
}
