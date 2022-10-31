package main

import (
	"fmt"
	"net"
	"net/url"

	"github.com/talostrading/sonic"
	"github.com/talostrading/sonic/codec/websocket"
)

func main() {
	ioc := sonic.MustIO()
	defer ioc.Close()

	client, err := websocket.NewWebsocketStream(ioc)
	if err != nil {
		panic(err)
	}

	uri, err := url.Parse("ws://localhost:8080")
	if err != nil {
		panic(err)
	}

	nc, err := net.Dial("tcp", uri.Host)
	if err != nil {
		panic(err)
	}

	conn, err := sonic.AdaptNetConn(ioc, nc)
	if err != nil {
		panic(err)
	}

	client.AsyncHandshake(conn, uri, func(err error) {
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
