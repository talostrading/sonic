package main

import (
	"fmt"
	"github.com/talostrading/sonic"
	"github.com/talostrading/sonic/codec/websocket"
	"net"
	"net/url"
)

var (
	msg = []byte("hello")
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

	fmt.Println("client writing", string(msg))

	client.AsyncHandshake(conn, uri, func(err error) {
		if err != nil {
			panic(err)
		}

		b := make([]byte, 32)
		var onWrite func(error)
		onWrite = func(err error) {
			if err != nil {
				panic(err)
			}

			b = b[:cap(b)]
			client.AsyncNextMessage(b, func(err error, n int, mt websocket.MessageType) {
				if err != nil {
					panic(err)
				}

				b = b[:n]
				client.AsyncWrite(msg, websocket.TypeText, onWrite)
			})
		}
		client.AsyncWrite(msg, websocket.TypeText, onWrite)
	})

	for {
		ioc.PollOne()
	}
}
