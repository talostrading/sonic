package sonic

import (
	"net"
	"testing"

	"github.com/talostrading/sonic/internal"
)

func TestTCPClientDefaultOpts(t *testing.T) {
	mark := make(chan struct{}, 1)
	defer func() { <-mark }()
	go func() {
		ln, err := net.Listen("tcp", "localhost:10100")
		if err != nil {
			panic(err)
		}
		defer func() {
			ln.Close()
			mark <- struct{}{}
		}()
		mark <- struct{}{}

		conn, err := ln.Accept()
		if err != nil {
			panic(err)
		}
		defer conn.Close()

		<-mark
	}()
	<-mark

	ioc := MustIO()
	defer ioc.Close()

	conn, err := Dial(ioc, "tcp", "localhost:10100")
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	nonblocking, err := internal.IsNonblocking(conn.RawFd())
	if err != nil {
		t.Fatal(err)
	}
	if !nonblocking {
		t.Fatal("sonic conn should be nonblocking by default")
	}

	noDelay, err := internal.IsNoDelay(conn.RawFd())
	if err != nil {
		t.Fatal(err)
	}
	if noDelay {
		t.Fatal("sonic tcp conn should not have Nagle's algorithm off by default")
	}

	mark <- struct{}{}
}
