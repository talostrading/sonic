package main

import (
	"fmt"

	"github.com/talostrading/sonic"
)

func main() {
	ioc := sonic.MustIO(-1)

	listener, err := sonic.Listen(ioc, "tcp", ":8080")
	if err != nil {
		panic(err)
	}

	conn, err := listener.Accept()
	if err != nil {
		panic(err)
	}

	conn.AsyncWrite([]byte("hello, sonic!"), func(err error, n int) {
		if err != nil {
			panic(err)
		}
		fmt.Println("wrote", n, "bytes")
	})

	ioc.Run()
}
