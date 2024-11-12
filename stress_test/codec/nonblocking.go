package main

import (
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/talostrading/sonic"
	"github.com/talostrading/sonic/codec/frame"
)

var (
	addr = flag.String("addr", "localhost:8080", "port to connect to")
	sum  = 0
)

func main() {
	flag.Parse()

	src := sonic.NewByteBuffer()
	dst := sonic.NewByteBuffer()
	codec := frame.NewCodec(src)

	ioc := sonic.MustIO()
	defer ioc.Close()

	conn, err := sonic.Dial(ioc, "tcp", *addr)
	if err != nil {
		panic(err)
	}
	defer conn.Close()
	log.Print("client connected, starting to read")
	cc, err := sonic.NewCodecConn[[]byte, []byte](conn, codec, src,
		dst)
	if err != nil {
		panic(err)
	}

	ticker, err := sonic.NewTimer(ioc)
	if err != nil {
		panic(err)
	}
	defer ticker.Close()
	if err := ticker.ScheduleRepeating(time.Second, func() {
		log.Print("tick")
	}); err != nil {
		panic(err)
	}

	var on func(error, []byte)
	on = func(err error, payload []byte) {
		if err != nil {
			panic(err)
		} else {
			if string(payload) != "hello, world!" {
				panic(fmt.Errorf("invalid payload %s", string(payload)))
			}
			// simulate some work
			n := 10
			for i := 0; i < 500_000; i++ {
				n += i
			}
			sum += n
			cc.AsyncReadNext(on)
		}
	}
	cc.AsyncReadNext(on)

	nRead := 0
	start := time.Now()
	for time.Since(start).Minutes() < 10 {
		n, _ := ioc.PollOne()
		if n > 0 {
			nRead += n
		}
	}
	fmt.Println(nRead)
	fmt.Println(sum)
}
