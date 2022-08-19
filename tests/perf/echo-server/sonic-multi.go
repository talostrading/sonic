package main

import (
	"flag"
	"fmt"
	"runtime"
	"runtime/debug"
	"sync"

	"github.com/talostrading/sonic"
	"github.com/talostrading/sonic/sonicopts"
)

var (
	hot     = flag.Bool("hot", false, "if set, sonic busy waits for events")
	nthread = flag.Int("threads", 1, "number of threads to use for echoing messages")
)

func main() {
	flag.Parse()

	debug.SetGCPercent(-1)
	runtime.GOMAXPROCS(*nthread + 1) // +1 for the main thread

	var wg sync.WaitGroup

	for i := 0; i < *nthread; i++ {
		// We are creating `nthread` threads, each with their own listener. Each
		// listener binds to the same (host,port). In Linux, incoming connections
		// are distributed evenly accross listening threads. In BSD, there is
		// no connection distribution - a single listener thread accepts all
		// connections.

		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			listen(id)
		}(i + 1)
	}

	wg.Wait()
}

func listen(id int) {
	fmt.Println("created listener", id)

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	ioc := sonic.MustIO()
	defer ioc.Close()

	ln, err := sonic.Listen(
		ioc,
		"tcp",
		"127.0.0.1:8080",
		sonicopts.ReuseAddr(true),
		sonicopts.ReusePort(true),
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
			fmt.Printf("listener %d accepted connection %d\n", id, connId)
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
		if err == nil {
			conn.AsyncWrite(b[:n], func(err error, n int) {
				if err == nil {
					conn.AsyncRead(b[:cap(b)], onAsyncRead)
				}
			})
		}
	}
	conn.AsyncRead(b[:], onAsyncRead)
}
