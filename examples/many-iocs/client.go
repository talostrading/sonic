package main

import (
	"flag"
	"fmt"
	"runtime"
	"sync"
	"time"

	"github.com/talostrading/sonic"
)

var (
	addr = flag.String("addr", "localhost:8080", "server address")
)

// This example builds up two go-routines which print a message every second
// along with a third client go-routine which reads a message from server.go
// asynchronously.
//
// The client go-routine has its own io_context and does not call
// runtime.LockOSThread(). Even though under the hood a call to kqueue/epoll
// is made with an indefinite timeout (which in C yields the current thread)
// the other two ticker go-routines are not affected.
//
// This proves that each go-routine can carry asynchronous calls to completion
// with its own io_context safely, without starving other go-routines.

func main() {
	n := 2
	runtime.GOMAXPROCS(n) // run at most 2 go-routines in parallel

	var wg sync.WaitGroup

	for i := 0; i < n; i++ {
		id := i
		go func() {
			wg.Add(1)
			defer wg.Done()

			t := time.NewTicker(time.Millisecond)

			for {
				select {
				case <-t.C:
					fmt.Println("goroutine", id, "ticked", time.Now())
				default:
				}
			}
		}()
	}

	go func() {
		wg.Add(1)
		defer wg.Done()

		ioc := sonic.MustIO()
		defer ioc.Close()

		conn, err := sonic.Dial(ioc, "tcp", *addr)
		if err != nil {
			panic(err)
		}

		b := make([]byte, 128)
		var onAsyncRead sonic.AsyncCallback
		onAsyncRead = func(err error, n int) {
			if err != nil {
				panic(err)
			}

			b = b[:n]
			fmt.Println("client", "read", string(b), n)

			b = b[:cap(b)]
			conn.AsyncRead(b, onAsyncRead)
		}
		conn.AsyncRead(b, onAsyncRead)

		ioc.Run()
	}()

	wg.Wait()
}
