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
  "params": [ "bnbbtc@depth@0ms" ]
}
`)

var b = make([]byte, 512*1024) // contains websocket payloads

func run(stream websocket.Stream) {
    stream.AsyncHandshake("wss://stream.binance.com:9443/ws", func(err error) {
		onHandshake(err, stream)
	})
}

func onHandshake(err error, stream websocket.Stream) {
	if err != nil {
		panic(err)
	} else {
		stream.AsyncWrite(subscriptionMessage, websocket.TypeText, func(err error) {
			onWrite(err, stream)
		})
	}
}

func onWrite(err error, stream websocket.Stream) {
	if err != nil {
		panic(err)
	} else {
		readLoop(stream)
	}
}

func readLoop(stream websocket.Stream) {
	var onRead websocket.AsyncMessageHandler
	onRead = func(err error, n int, _ websocket.MessageType) {
		if err != nil {
			panic(err)
		} else {
			b = b[:n]
			fmt.Println(string(b))
			b = b[:cap(b)]

			stream.AsyncNextMessage(b, onRead)
		}
	}
	stream.AsyncNextMessage(b, onRead)
}

func main() {
	ioc := sonic.MustIO()
	defer ioc.Close()

	stream, err := websocket.NewWebsocketStream(ioc, &tls.Config{}, websocket.RoleClient)
	if err != nil {
		panic(err)
	}

	run(stream)

	ioc.Run()
}
