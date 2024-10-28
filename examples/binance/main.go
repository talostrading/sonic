package main

import (
	"crypto/tls"
	"fmt"

	"github.com/talostrading/sonic"
	"github.com/talostrading/sonic/codec/websocket"
)

var subscriptionMessage = []byte(
	`
{
  "id": 1,
  "method": "SUBSCRIBE",
  "params": [ "bnbbtc@depth" ]
}
`)

func main() {
	ioc := sonic.MustIO()
	defer ioc.Close()

	stream, err := websocket.NewWebsocketStream(ioc, &tls.Config{}, websocket.RoleClient)
	if err != nil {
		panic(err)
	}

	stream.AsyncHandshake("wss://stream.binance.com:9443/ws", func(err error) {
		if err != nil {
			panic(err)
		}

		stream.AsyncWrite(subscriptionMessage, websocket.TypeText, func(err error) {
			if err != nil {
				panic(err)
			}

			var (
				b      [1024 * 512]byte
				onRead websocket.AsyncMessageCallback
			)
			onRead = func(err error, n int, _ websocket.MessageType) {
				if err != nil {
					panic(err)
				}

				fmt.Println(string(b[:n]))
				stream.AsyncNextMessage(b[:], onRead)
			}
			stream.AsyncNextMessage(b[:], onRead)
		})
	})

	ioc.Run()
}
