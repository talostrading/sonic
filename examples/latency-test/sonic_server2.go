package main

import (
	"flag"
	"log"

	"github.com/csdenboer/sonic"
	"github.com/csdenboer/sonic/sonicopts"
)

var (
	addr = flag.String("addr", "localhost:8080", "address to connect to")
	hot  = flag.Bool("hot", true, "if true, busy-wait for events")
)

func main() {
	flag.Parse()

	ioc := sonic.MustIO()
	defer ioc.Close()

	ln, err := sonic.Listen(ioc, "tcp", *addr, sonicopts.Nonblocking(true))
	if err != nil {
		panic(err)
	}
	defer ln.Close()

	var (
		onAccept sonic.AcceptCallback
		b        = make([]byte, 8)
	)

	onAccept = func(err error, conn sonic.Conn) {
		if err != nil {
			panic(err)
		}
		ln.AsyncAccept(onAccept)

		log.Printf(
			"accepted conn local=%s remote=%s",
			conn.LocalAddr(), conn.RemoteAddr())

		var (
			onRead  sonic.AsyncCallback
			onWrite sonic.AsyncCallback
		)

		onWrite = func(err error, _ int) {
			if err != nil {
				panic(err)
			}
			conn.AsyncRead(b, onRead)
		}

		onRead = func(err error, _ int) {
			if err != nil {
				panic(err)
			} else {
				conn.AsyncWrite(b, onWrite)
			}
		}

		conn.AsyncRead(b, onRead)
	}

	ln.AsyncAccept(onAccept)

	if *hot {
		for {
			ioc.PollOne()
		}
	} else {
		ioc.Run()
	}
}
