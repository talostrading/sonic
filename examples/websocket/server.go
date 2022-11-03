package main

import (
	"fmt"
	"github.com/talostrading/sonic"
	"github.com/talostrading/sonic/codec/websocket"
	"github.com/talostrading/sonic/sonicopts"
	"net/url"
	"time"
)

var (
	reportDur = 5 * time.Second
	count     = 0
)

func main() {
	ioc := sonic.MustIO()
	defer ioc.Close()

	ticker, err := sonic.NewTimer(ioc)
	err = ticker.ScheduleRepeating(reportDur, func() {
		fmt.Println("received", count, "in", reportDur)
		rate := reportDur / time.Duration(count)
		fmt.Println("rate", rate)
		count = 0
	})
	if err != nil {
		panic(err)
	}

	url, err := url.Parse("ws://localhost:8080")
	if err != nil {
		panic(err)
	}

	ln, err := sonic.Listen(ioc, "tcp", url.Host, sonicopts.Nonblocking(true))
	if err != nil {
		panic(err)
	}

	ln.AsyncAccept(func(err error, conn sonic.Conn) {
		if err != nil {
			panic(err)
		}

		ws, err := websocket.NewWebsocketStream(ioc)
		if err != nil {
			panic(err)
		}

		ws.AsyncAccept(conn, url, func(err error) {
			if err != nil {
				panic(err)
			}

			b := make([]byte, 32)
			var onMessage websocket.AsyncMessageHandler
			onMessage = func(err error, n int, _ websocket.MessageType) {
				if err != nil {
					panic(err)
				}

				count++

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
	})

	for {
		ioc.PollOne()
	}
}
