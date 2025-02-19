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
		"jsonrpc": "2.0",
		"id": 42,
		"method": "public/subscribe",
		"params": {
			"channels": [
				"book.BTC-PERPETUAL.100ms"
			]
		}
	}
	`)
)

func main() {
	flag.Parse()

	ioc := sonic.MustIO()
	defer ioc.Close()

	ioLatency := util.NewOnlineStats()

	ws, err := websocket.NewWebsocketStream(ioc, &tls.Config{}, websocket.RoleClient)
	if err != nil {
		panic(err)
	}

	ws.AsyncHandshake("wss://test.deribit.com/ws/api/v2", func(err error) {
		if err != nil {
			panic(err)
		}

		ws.AsyncWrite(subscriptionMessage, websocket.TypeText, func(err error) {
			if err != nil {
				panic(err)
			}

			var (
				b      [1024 * 512]byte
				onRead websocket.AsyncMessageDirectCallback
			)
			onRead = func(err error, mt websocket.MessageType, pl ...[]byte) {
				if err != nil {
					panic(err)
				}

				asm := websocket.NewFrameAssembler(pl...)

				asm.ReassembleInto(b[:])
				n := asm.Length()

				if *verbose {
					fmt.Println(string(b[:n]))
				}
				ws.AsyncNextMessageDirect(onRead)
			}
			ws.AsyncNextMessageDirect(onRead)
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
			ioLatency.Add(float64(time.Since(start).Microseconds()))
		}
	}
}
