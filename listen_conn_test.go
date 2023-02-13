package sonic

import (
	"github.com/talostrading/sonic/internal"
	"net"
	"testing"
)

func TestTCPConnListenerDefaultOpts(t *testing.T) {
	mark := make(chan struct{}, 1)
	go func() {
		<-mark
		conn, err := net.Dial("tcp", "localhost:10100")
		if err != nil {
			panic(err)
		}
		defer conn.Close()
		<-mark
	}()

	ioc := MustIO()
	defer ioc.Close()

	ln, err := Listen(ioc, "tcp", "localhost:10100")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	nonblocking, err := internal.IsNonblocking(ln.RawFd())
	if err != nil {
		t.Fatal(err)
	}

	nodelay, err := internal.IsNoDelay(ln.RawFd())
	if err != nil {
		t.Fatal(err)
	}

	if nonblocking {
		t.Fatal("listener should be blocking")
	}

	if nodelay {
		t.Fatal("listener should not have Nagle's off")
	}

	{
		mark <- struct{}{}

		conn, err := ln.Accept()
		if err != nil {
			t.Fatal(err)
		}
		defer conn.Close()

		nonblocking, err := internal.IsNonblocking(conn.RawFd())
		if err != nil {
			t.Fatal(err)
		}

		nodelay, err := internal.IsNoDelay(conn.RawFd())
		if err != nil {
			t.Fatal(err)
		}

		if !nonblocking {
			t.Fatal("accepted connection should be nonblocking")
		}

		if nodelay {
			t.Fatal("accepted connection should not have Nagle's off")
		}

		mark <- struct{}{}
	}
}
