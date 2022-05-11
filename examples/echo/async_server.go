package main

import (
	"fmt"

	"github.com/talostrading/sonic"
	"github.com/talostrading/sonic/sonicopts"
)

func onAccept(err error, conn sonic.Conn) {
	if err != nil {
		panic(err)
	}

	conn.AsyncWrite([]byte("hello, sonic!"), func(err error, n int) {
		if err != nil {
			panic(err)
		}
		fmt.Println("wrote", n, "bytes")
	})
}

func main() {
	ioc := sonic.MustIO(-1)

	listener, err := sonic.Listen(ioc, "tcp", ":8080", sonicopts.Nonblocking(true))
	if err != nil {
		panic(err)
	}

	listener.AsyncAccept(onAccept)

	fmt.Println("this is printed before accepting anything, as we do not block")

	ioc.RunPending()
}
