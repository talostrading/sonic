package main

import (
	"encoding/binary"
	"flag"
	"github.com/talostrading/sonic"
	"github.com/talostrading/sonic/codec/frame"
	"log"
)

var (
	addr = flag.String("addr", "localhost:8080", "addr")
	rate = flag.Duration("rate", 0, "rate at which to send")
)

func main() {
	flag.Parse()

	ioc := sonic.MustIO()
	defer ioc.Close()

	ln, err := sonic.Listen(ioc, "tcp", *addr)

	conn, err := ln.Accept()
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	log.Print("server connected, starting to write")
	payload := []byte("hello, world!")
	b := make([]byte, frame.HeaderLen+len(payload))
	binary.BigEndian.PutUint32(b[:frame.HeaderLen], uint32(len(payload)))
	copied := copy(b[frame.HeaderLen:], payload)
	if copied != len(payload) {
		panic("did not copy enough")
	}

	if *rate == 0 {
		log.Print("sending as fast as possible")
		var fn func(error, int)
		fn = func(err error, n int) {
			if err != nil {
				panic(err)
			} else {
				conn.AsyncWriteAll(b, fn)
			}
		}
		conn.AsyncWriteAll(b, fn)
	} else {
		log.Printf("sending every %s", *rate)
		t, err := sonic.NewTimer(ioc)
		if err != nil {
			panic(err)
		}
		t.ScheduleRepeating(*rate, func() {
			conn.AsyncWriteAll(b, func(err error, _ int) {
				if err != nil {
					panic(err)
				}
			})
		})
	}

	for {
		ioc.PollOne()
	}
}
