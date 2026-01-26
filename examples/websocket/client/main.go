package main

import (
	"fmt"

	"github.com/talostrading/sonic"
	"github.com/talostrading/sonic/codec/websocket"
)

func main() {
	ioc := sonic.MustIO()
	defer ioc.Close()

	client, err := websocket.NewWebsocketStream(ioc, nil, websocket.RoleClient)
	if err != nil {
		panic(err)
	}

	client.AsyncHandshake("ws://localhost:8080", func(err error) {
		if err != nil {
			panic(err)
		}
		client.AsyncWrite([]byte("hello"), websocket.TypeText, func(err error) {
			if err != nil {
				panic(err)
			}

			var b [128]byte
			client.AsyncNextMessage(b[:], func(err error, n int, mt websocket.MessageType) {
				if err != nil {
					panic(err)
				}
				fmt.Println("read", n, "bytes", string(b[:n]), err)
			})
		})
	})

	ioc.Run()
}
