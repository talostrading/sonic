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
