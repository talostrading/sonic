package sonicwebsocket

import (
	"fmt"
	"testing"

	"github.com/talostrading/sonic"
)

func TestHandshake2(t *testing.T) {
	server := &testServer{}
	server.AsyncAccept("localhost:8080", func(err error) {
		if err != nil {
			t.Fatal(err)
		}
	})

	ioc := sonic.MustIO()
	defer ioc.Close()

	ws, err := NewWebsocketStream(ioc, nil, RoleClient)
	if err != nil {
		t.Fatal(err)
	}

	ws.AsyncHandshake("ws://localhost:8080", func(err error) {
		if err != nil {
			t.Fatal(err)
		} else {
			fmt.Println("handshake succeeded")
		}
	})

	ioc.RunOne()
}
