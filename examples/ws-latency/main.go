package main

import (
	"crypto/tls"
	"encoding/binary"
	"flag"
	"log"
	"runtime"
	"runtime/debug"
	"time"

	"github.com/talostrading/sonic"
	"github.com/talostrading/sonic/codec/websocket"
	"github.com/talostrading/sonic/util"
)

// Sends a websocket ping with time t1 in the payload. Per the WebSocket protocol, the peer must reply with a pong
// immediately, with a payload equal to the ping's. On a pong, time t2 is taken again and time t1 is read from the payload.
// t2 - t1 is then printed to the console - this is the application layer RTT between the peers.

// example: go run main.go -v=false -addr="wss://stream.binance.com:9443/ws" -s="{\"id\":1,\"method\":\"SUBSCRIBE\",\"params\":[\"bnbbtc@depth\"]}" -n 2

var (
	addr    = flag.String("addr", "wss://stream.binance.com:9443/ws", "address")
	n       = flag.Int("n", 1, "number of websocket streams")
	verbose = flag.Bool("v", false, "if true, print every inbound message")
	subMsg  = flag.String("s", "", "subscription message") // optional, if empty, we simply start reading directly
)

func main() {
	flag.Parse()

	runtime.LockOSThread()
	debug.SetGCPercent(-1)

	ioc := sonic.MustIO()
	defer ioc.Close()

	for i := 0; i < *n; i++ {
		stream, err := websocket.NewWebsocketStream(ioc, &tls.Config{}, websocket.RoleClient)
		if err != nil {
			panic(err)
		}

		log.Println("websocket", i, "connecting to", *addr)
		stream.AsyncHandshake(*addr, func(err error) {
			if err != nil {
				panic(err)
			}

			log.Println("websocket", i, "connected to", *addr)

			first := true

			b := make([]byte, 4096)
			var onRead func(error, int, websocket.MessageType)
			onRead = func(err error, n int, _ websocket.MessageType) {
				if err != nil {
					panic(err)
				}
				if *verbose || first {
					if first {
						log.Println("websocket", i, "read first message", string(b))
						first = false
					} else {
						log.Println("websocket", i, "read", string(b))
					}
				}
				stream.AsyncNextMessage(b, onRead)
			}

			if *subMsg != "" {
				stream.AsyncWrite([]byte(*subMsg), websocket.TypeText, func(err error) {
					if err != nil {
						panic(err)
					}
					stream.AsyncNextMessage(b, onRead)
				})
			} else {
				stream.AsyncNextMessage(b, onRead)
			}

			pingPayload := make([]byte, 48)
			pingTimer, err := sonic.NewTimer(ioc)
			pingTimer.ScheduleRepeating(5*time.Second, func() {
				binary.LittleEndian.PutUint64(pingPayload, uint64(util.GetMonoTimeNanos()))
				stream.AsyncWrite(pingPayload, websocket.TypePing, func(err error) {
					if err != nil {
						panic(err)
					}
				})
			})

			stream.SetControlCallback(func(mt websocket.MessageType, b []byte) {
				if mt == websocket.TypePong {
					diff := util.GetMonoTimeNanos() - int64(binary.LittleEndian.Uint64(b))
					log.Println("websocket", i, "RTT:", time.Duration(diff)*time.Nanosecond)
				}
			})
		})
	}

	for {
		ioc.PollOne()
	}
}
