package main

import (
	"crypto/tls"
	"flag"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/talostrading/sonic"
	"github.com/talostrading/sonic/codec/websocket"
	"github.com/talostrading/sonic/util"
)

// example: go run main.go -t="linear" -v=false

var (
	streamType = flag.String("t", "spot", "spot/inverse/linear")
	verbose    = flag.Bool("v", false, "if true then payloads are also printed, otherwise only rtts")
	addr       = flag.String("a", "", "if non-empty then the address we connect to. Must match the type")
)

var (
	subMsg []byte
)

var b = make([]byte, 512*1024) // contains websocket payloads

func run(stream websocket.Stream) {
	fmt.Println("connecting to", *addr)
	stream.AsyncHandshake(*addr, func(err error) {
		onHandshake(err, stream)
	})
}

func onHandshake(err error, stream websocket.Stream) {
	if err != nil {
		panic(err)
	} else {
		fmt.Println("connected, subscribing with", string(subMsg))
		stream.AsyncWrite(subMsg, websocket.TypeText, func(err error) {
			onWrite(err, stream)
		})
	}
}

func onWrite(err error, stream websocket.Stream) {
	if err != nil {
		panic(err)
	} else {
		fmt.Println("subscribed")
		readLoop(stream)
	}
}

func readLoop(stream websocket.Stream) {
	var onRead websocket.AsyncMessageHandler
	onRead = func(err error, n int, mt websocket.MessageType) {
		if err != nil {
			panic(err)
		} else {
			b = b[:n]
			if *verbose {
				fmt.Println(string(b))
			}
			b = b[:cap(b)]

			stream.AsyncNextMessage(b, onRead)
		}
	}
	stream.AsyncNextMessage(b, onRead)
}



func main() {
	flag.Parse()
	var all_rtts []time.Duration

	if *streamType == "spot" {
		subMsg = []byte(fmt.Sprintf(
			`
{
  "id": 1,
  "method": "SUBSCRIBE",
  "params": [ "btcusd@depth@0ms" ]
}
`,
		))
		if *addr == "" {
			*addr = "wss://stream.binance.com:9443/ws"
		}
		fmt.Println("connecting to spot", *addr)
	} else if *streamType == "inverse" {
		subMsg = []byte(fmt.Sprintf(
			`
{
  "id": 1,
  "method": "SUBSCRIBE",
  "params": [ "btcusd@depth@0ms" ]
}
`,
		))
		if *addr == "" {
			*addr = "wss://dstream.binance.com/ws"
		}
		fmt.Println("connecting to inverse", *addr)
	} else if *streamType == "linear" {
		subMsg = []byte(fmt.Sprintf(
			`
{
  "id": 1,
  "method": "SUBSCRIBE",
  "params": [ "btcusdt@depth@0ms" ]
}
`,
		))
		if *addr == "" {
			*addr = "wss://fstream.binance.com/ws"
		}
		fmt.Println("connecting to linear", *addr)
	} else {
		panic("invalid stream type. choices are: spot, inverse, linear")
	}

	ioc := sonic.MustIO()
	defer ioc.Close()

	stream, err := websocket.NewWebsocketStream(ioc, &tls.Config{InsecureSkipVerify: true}, websocket.RoleClient)
	if err != nil {
		panic(err)
	}

	run(stream)

	t, err := sonic.NewTimer(ioc)
	if err != nil {
		panic(err)
	}
	hist := util.NewTtyHist(util.TtyHistOpts{
		Name:      "sample",
		Scale:     "ms",
		N:         50,
		MinPct:    0.1,
		Min:       1,
		Max:       100000000,
		Precision: 1,
		Writer:    os.Stdout,
	})
	err = t.ScheduleRepeating(5*time.Second, func() {
		now := []byte(strconv.FormatInt(time.Now().UnixMicro(), 10))
		stream.AsyncWrite(now, websocket.TypePing, func(err error) {
			if err != nil {
				fmt.Println("could not write ping")
				panic(err)
			}
		})
	})
	stream.SetControlCallback(func(mt websocket.MessageType, b []byte) {
		if mt == websocket.TypePong {
			ts, _ := strconv.ParseInt(string(b), 10, 64)
			rtt := time.Now().Sub(time.UnixMicro(ts))
			all_rtts = append(all_rtts, rtt)

			hist.Add(rtt.Milliseconds())
			fmt.Println("Last RTT", rtt)

		}
	})
	if err != nil {
		panic(err)
	}

	ioc.Run()
}
