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

	if expect := StateTerminated; ws.State() != expect {
		t.Fatalf("wrong websocket state expected=%s given=%s", expect, ws.State())
	}

	called := false
	ws.AsyncHandshake("ws://localhost:8080", func(err error) {
		called = true

		if err != nil {
			t.Fatal(err)
		} else {

			if expect := StateActive; ws.State() != expect {
				t.Fatalf("wrong websocket state expected=%s given=%s", expect, ws.State())
			}
		}
	})

	ioc.RunOne()

	if !called {
		t.Fatal("failed handshake")
	}

	srv.Close() // TODO
}

func TestAsyncReadSingleFrame(t *testing.T) {
	expected := "hello"

	srv := &testServer{}
	go func() {
		err := srv.Accept("localhost:8081")
		if err != nil {
			panic(err)
		}

		_, err = srv.Write([]byte(expected), 1, true)
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

	called := false
	ws.AsyncHandshake("ws://localhost:8081", func(err error) {
		if err != nil {
			t.Fatal(err)
		} else {
			b := make([]byte, 4096)
			ws.AsyncReadSome(b, func(err error, n int, mt MessageType) {
				called = true

				if err != nil {
					t.Fatal(err)
				} else {
					if expect := StateActive; ws.State() != expect {
						t.Fatalf("wrong websocket state expected=%s given=%s", expect, ws.State())
					}

					b = b[:n]
					if given := string(b); given != expected {
						t.Fatalf("invalid read expected=%s given=%s", expected, given)
					}
				}
			})
		}
	})

	ioc.RunOne()

	if !called {
		t.Fatal("did not read from server")
	}

	srv.Close() // TODO
}
