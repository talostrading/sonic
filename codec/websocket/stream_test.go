package websocket

import (
	"testing"

	"github.com/talostrading/sonic"
)

func TestClientHandshake(t *testing.T) {
	srv := &MockServer{}

	go func() {
		defer func() {
			srv.Close()
		}()

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

	if expect := StateHandshake; ws.State() != expect {
		t.Fatalf("wrong state expected=%s given=%s", expect, ws.State())
	}

	ws.AsyncHandshake("ws://localhost:8080", func(err error) {
		if err != nil {
			if expect := StateTerminated; ws.State() != expect {
				t.Fatalf("failed handshake with wrong state expected=%s given=%s", expect, ws.State())
			} else {
				t.Fatal(err)
			}
		} else {
			if expect := StateActive; ws.State() != expect {
				t.Fatalf("wrong state expected=%s given=%s", expect, ws.State())
			}
		}
	})

	for {
		ioc.RunOne()
		if srv.IsClosed() {
			break
		}
	}
}
