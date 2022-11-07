package main

import (
	"flag"
	"fmt"
	"net/url"

	"github.com/talostrading/sonic"
	"github.com/talostrading/sonic/codec/websocket"
	"github.com/talostrading/sonic/sonicopts"
)

func main() {
	flag.Parse()

	ioc := sonic.MustIO()
	defer ioc.Close()

	url, err := url.Parse("ws://localhost:8080")
	if err != nil {
		panic(err)
	}

	ln, err := sonic.Listen(ioc, "tcp", url.Host, sonicopts.Nonblocking(true))
	if err != nil {
		panic(err)
	}

	nconn := 0

	var onAsyncAccept sonic.AcceptCallback
	onAsyncAccept = func(err error, conn sonic.Conn) {
		if err != nil {
			panic(err)
		}

		nconn++
		fmt.Println("accepted conn", nconn)

		ln.AsyncAccept(onAsyncAccept)

		ws, err := websocket.NewWebsocketStream(ioc)
		if err != nil {
			panic(err)
		}

		ws.AsyncAccept(conn, url, func(err error) {
			if err != nil {
				panic(err)
			}

			b := make([]byte, 128)
			var onMessage websocket.AsyncMessageHandler
			onMessage = func(err error, n int, _ websocket.MessageType) {
				if err != nil {
					panic(err)
				}

				b = b[:n]
				ws.AsyncWrite(b, websocket.TypeText, func(err error) {
					if err != nil {
						panic(err)
					}

					b = b[:cap(b)]
					ws.AsyncNextMessage(b, onMessage)
				})
			}
			ws.AsyncNextMessage(b, onMessage)
		})

	}
	ln.AsyncAccept(onAsyncAccept)

	for {
		ioc.PollOne()
	}
}
