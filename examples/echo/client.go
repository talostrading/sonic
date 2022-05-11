package main

import (
	"fmt"

	"github.com/talostrading/sonic"
)

func main() {
	ioc := sonic.MustIO(-1)

	conn, err := sonic.Dial(ioc, "tcp", "localhost:8080")
	if err != nil {
		panic(err)
	}

	b := make([]byte, 20)
	conn.AsyncRead(b, func(err error, n int) {
		if err != nil {
			panic(err)
		}
		fmt.Println("read", n, "bytes:", string(b))
	})

	ioc.RunPending()
}
