package sonic

import (
	"github.com/talostrading/sonic/sonicopts"
	"net"
	"testing"
)

func TestReconnectingConn_Reconnect(t *testing.T) {
	tries := 2

	flag := make(chan struct{}, 1)
	defer func() {
		<-flag
	}()

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

		for i := 0; i < tries; i++ {
			conn, err := ln.Accept()
			if err != nil {
				panic(err)
			}

			_, err = conn.Write([]byte("hello"))
			if err != nil {
				panic(err)
			}

			conn.Close()
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
		sonicopts.Nonblocking(false), // makes it easier to test because we cannot get ErrWouldBlock
		sonicopts.UseNetConn(false))
	if err != nil {
		t.Fatal(err)
	}
	reconnects := 0
	conn.SetOnReconnect(func() {
		reconnects++
	})

	b := make([]byte, 128)

outer:
	for {
		b = b[:cap(b)]
		n, err := conn.Read(b)
		if err == nil {
			b = b[:n]
			if string(b) != "hello" {
				t.Fatal("wrong read")
			}
		}

		ioc.PollOne()

		select {
		case <-flag:
			break outer
		default:
		}
	}

	if reconnects != tries-1 {
		t.Fatal("did not reconnect correctly")
	}
}
