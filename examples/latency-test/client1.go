package main

import (
	"flag"

	"github.com/csdenboer/sonic"
)

var addr = flag.String("addr", "localhost:8080", "address to connect to")

func main() {
	flag.Parse()

	ioc := sonic.MustIO()
	defer ioc.Close()

	conn, err := sonic.Dial(ioc, "tcp", *addr)
	if err != nil {
		panic(err)
	}

	b := make([]byte, 8)

	var (
		onRead  sonic.AsyncCallback
		onWrite sonic.AsyncCallback
	)

	onRead = func(err error, _ int) {
		if err != nil {
			panic(err)
		} else {
			conn.AsyncWrite(b, onWrite)
		}
	}

	onWrite = func(err error, _ int) {
		if err != nil {
			panic(err)
		}
		conn.AsyncRead(b, onRead)
	}

	conn.AsyncRead(b, onRead)

	for {
		ioc.PollOne()
	}
}
