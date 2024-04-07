package main

import (
	"flag"
	"runtime"
	"runtime/debug"

	"github.com/talostrading/sonic"
)

var n = flag.Int("n", 10, "number of connections")

func main() {
	debug.SetGCPercent(-1)
	runtime.LockOSThread()

	flag.Parse()

	ioc := sonic.MustIO()

	b := make([]byte, 128)

	for i := 0; i < *n; i++ {
		conn, err := sonic.Dial(ioc, "tcp", "localhost:1234")
		if err != nil {
			panic(err)
		}
		var onRead func(error, int)
		var onWrite func(error, int)
		onRead = func(err error, n int) {
			if err != nil {
				panic(err)
			}
			conn.AsyncWriteAll(b, onWrite)
		}
		onWrite = func(err error, n int) {
			if err != nil {
				panic(err)
			}
			conn.AsyncReadAll(b, onRead)
		}
		conn.AsyncReadAll(b, onRead)
	}

	for {
		ioc.RunOne()
	}
}
