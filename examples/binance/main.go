package main

import (
	"crypto/tls"
	"flag"
	"fmt"
	"time"

	"github.com/talostrading/sonic"
	"github.com/talostrading/sonic/codec/websocket"
	"github.com/talostrading/sonic/util"
)

var (
	verbose = flag.Bool("v", false, "if true, websocket messages are printed")

	subscriptionMessage = []byte(`
{
  "id": 1,
  "method": "SUBSCRIBE",
  "params": [ "bnbbtc@depth" ]
}
`)
)

func main() {
	flag.Parse()

	ioc := sonic.MustIO()
	defer ioc.Close()

	ioLatency := util.NewOnlineStats()

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

				if *verbose {
					fmt.Println(string(b[:n]))
				}
				stream.AsyncNextMessage(b[:], onRead)
			}
			stream.AsyncNextMessage(b[:], onRead)
		})
	})

	eventsReceived := 0

	ioLatencyTimer, err := sonic.NewTimer(ioc)
	if err != nil {
		panic(err)
	}
	ioLatencyTimer.ScheduleRepeating(time.Second, func() {
		if eventsReceived > 0 {
			result := ioLatency.Result()
			fmt.Printf(
				"min/avg/max/stddev = %.2f/%.2f/%.2f/%.2f us from %d events\n",
				result.Min,
				result.Avg,
				result.Max,
				result.StdDev,
				eventsReceived,
			)
			ioLatency.Reset()
			eventsReceived = 0
		}
	})

	for {
		start := time.Now()
		n, _ := ioc.PollOne()
		if n > 0 {
			eventsReceived += n
			ioLatency.Add(float64(time.Now().Sub(start).Microseconds()))
		}
	}
}
