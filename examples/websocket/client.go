package main

import (
	"fmt"

	"github.com/csdenboer/sonic"
	"github.com/csdenboer/sonic/codec/websocket"
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
		} else {
			client.AsyncWrite([]byte("hello"), websocket.TypeText, func(err error) {
				if err != nil {
					panic(err)
				} else {
					b := make([]byte, 128)
					client.AsyncNextMessage(b, func(err error, n int, mt websocket.MessageType) {
						if err != nil {
							panic(err)
						} else {
							b = b[:n]
							fmt.Println("read", n, "bytes", string(b), err)
						}
					})
				}
			})
		}
	})

	for {
		ioc.RunOneFor(0) // poll
	}
}
