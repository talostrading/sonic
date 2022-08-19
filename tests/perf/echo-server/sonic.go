package main

import (
	"flag"
	"fmt"
	"runtime"
	"runtime/debug"

	"github.com/talostrading/sonic"
	"github.com/talostrading/sonic/sonicopts"
)

var (
	hot = flag.Bool("hot", false, "if set, sonic busy waits for events")
)

func main() {
	flag.Parse()

	debug.SetGCPercent(-1)

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	ioc := sonic.MustIO()
	defer ioc.Close()

	ln, err := sonic.Listen(
		ioc,
		"tcp",
		"127.0.0.1:8080",
		sonicopts.Nonblocking(true),
	)
	if err != nil {
		panic(err)
	}

	connId := 1

	var onAsyncAccept sonic.AcceptCallback
	onAsyncAccept = func(err error, conn sonic.Conn) {
		if err != nil {
			fmt.Println("cannot accept", err)
		} else {
			fmt.Printf("listener accepted connection %d\n", connId)
			handle(conn)
			connId++
			ln.AsyncAccept(onAsyncAccept)
		}
	}
	ln.AsyncAccept(onAsyncAccept)

	if *hot {
		for {
			ioc.PollOne()
		}
	} else {
		ioc.Run()
	}
}

func handle(conn sonic.Conn) {
	var b [1024]byte

	var onAsyncRead sonic.AsyncCallback
	onAsyncRead = func(err error, n int) {
		if err != nil {
			panic(err)
		} else {
			conn.AsyncWrite(b[:n], func(err error, n int) {
				if err != nil {
					panic(err)
				} else {
					conn.AsyncRead(b[:cap(b)], onAsyncRead)
				}
			})
		}
	}
	conn.AsyncRead(b[:], onAsyncRead)
}
