package sonicwebsocket

import (
	"testing"

	"github.com/talostrading/sonic"
)

func TestHandshake(t *testing.T) {
	srv := &testServer{}
	go func() {
		err := srv.Accept("localhost:8080")
		if err != nil {
			panic(err)
		}
	}()

	ioc := sonic.MustIO()
	defer ioc.Close()

	ws, err := NewWebsocketStream(ioc, nil, RoleClient)
	if err != nil {
		t.Fatal(err)
	}

	valid := false
	ws.AsyncHandshake("ws://localhost:8080", func(err error) {
		if err != nil {
			t.Fatal(err)
		} else {
			valid = true
			srv.Close() // TODO
		}
	})

	ioc.RunOne()

	if !valid {
		t.Fatal("failed handshake")
	}
}

func TestAsyncReadSome(t *testing.T) {
	msg := "hello"

	srv := &testServer{}
	go func() {
		err := srv.Accept("localhost:8081")
		if err != nil {
			panic(err)
		}

		_, err = srv.Write([]byte(msg), 1, true)
		if err != nil {
			panic(err)
		}
	}()

	ioc := sonic.MustIO()
	defer ioc.Close()

	ws, err := NewWebsocketStream(ioc, nil, RoleClient)
	if err != nil {
		t.Fatal(err)
	}

	valid := false
	ws.AsyncHandshake("ws://localhost:8081", func(err error) {
		if err != nil {
			t.Fatal(err)
		} else {
			b := make([]byte, 4096)
			ws.AsyncReadSome(b, func(err error, n int, mt MessageType) {
				valid = true
				if err != nil {
					t.Fatal(err)
				} else {
					b = b[:n]
					if sb := string(b); sb != msg {
						t.Fatalf("invalid read expected=%s given=%s", msg, string(b))
					}
					srv.Close() // TODO
				}
			})
		}
	})

	ioc.RunOne()

	if !valid {
		t.Fatal("did not read from server")
	}
}
