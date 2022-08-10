package main

import (
	"flag"
	"fmt"
	"time"

	"github.com/talostrading/sonic"
)

var (
	nclient = flag.Int("nclient", 10, "number of clients to spawn")
	addr    = flag.String("addr", "localhost:8080", "server address")
)

func main() {
	for i := 0; i < *nclient; i++ {
		id := i
		go func() {
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
				fmt.Println("client", id, "read", string(b), n)

				b = b[:cap(b)]
				conn.AsyncRead(b, onAsyncRead)
			}
			conn.AsyncRead(b, onAsyncRead)

			if id == 0 {
				for {
					ioc.PollOne()
				}
			} else {
				ioc.Run()
			}
		}()
	}

	for {
		time.Sleep(time.Second)
		fmt.Println(time.Now(), "main goroutine")
	}
}
