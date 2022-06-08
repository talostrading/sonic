package main

import (
	"fmt"

	"github.com/talostrading/sonic"
	"github.com/talostrading/sonic/sonicwebsocket"
)

func main() {
	ioc := sonic.MustIO()
	defer ioc.Close()

	client := sonicwebsocket.NewWebsocketStream(ioc, nil)
	client.AsyncHandshake("ws://localhost:8080", func(err error) {
		if err != nil {
			panic(err)
		} else {
			client.AsyncWrite([]byte("hello"), func(err error, n int) {
				if err != nil {
					panic(err)
				} else {
					fmt.Println("wrote", n, "bytes")
					buf := make([]byte, 128)
					client.AsyncReadSome(buf, func(err error, n int) {
						if err != nil {
							panic(err)
						} else {
							buf = buf[:n]
							fmt.Println("read", n, "bytes", string(buf), err)
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
