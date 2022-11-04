package sonic

import (
	"github.com/talostrading/sonic/sonicopts"
	"net"
	"testing"
)

func TestReconnectingConn_Read(t *testing.T) {
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
		flag <- struct{}{}
	}()
	<-flag

	ioc := MustIO()
	defer ioc.Close()

	conn, err := DialReconnecting(
		ioc,
		"tcp",
		"localhost:8080",
		sonicopts.Nonblocking(true),
		sonicopts.UseNetConn(false),
	)
	if err != nil {
		t.Fatal(err)
	}

	<-flag

	b := make([]byte, 128)
	n, err := conn.Read(b)
	if err != nil {
		t.Fatal(err)
	}
	b = b[:n]

	if string(b) != "hello" {
		t.Fatal("invalid message")
	}

	ioc.RunPending()
}
