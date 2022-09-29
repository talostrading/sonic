package main

import (
	"fmt"

	"github.com/talostrading/sonic"
	"github.com/talostrading/sonic/sonicopts"
)

func main() {
	ioc := sonic.MustIO()

	listener, err := sonic.Listen(ioc, "tcp", ":8082", sonicopts.Nonblocking(true))
	if err != nil {
		panic(err)
	}

	var onAccept sonic.AcceptCallback
	onAccept = func(err error, conn sonic.Conn) {
		listener.AsyncAccept(onAccept)

		if err != nil {
			fmt.Println("could not accept", err)
		}

		b := make([]byte, 32)
		conn.AsyncRead(b, func(err error, n int) {
			if err != nil {
				fmt.Println("could not read", err)
			} else {
				b = b[:n]
				fmt.Println("read:", string(b))

				conn.AsyncWrite(b, func(err error, n int) {
					if err != nil {
						fmt.Println("could not write", err)
					} else {
						fmt.Println("echoed")
					}
				})
			}
		})
	}

	listener.AsyncAccept(onAccept)

	fmt.Println("this is printed before accepting anything, as we do not block")

	ioc.Run()
}
