package main

import (
	"fmt"

	"github.com/talostrading/sonic"
)

func main() {
	ioc := sonic.MustIO()
	conn, err := sonic.Dial(ioc, "tcp", "google.com:80")
	if err != nil {
		panic(err)
	}

	buf := make([]byte, 4096)
	conn.AsyncRead(buf, func(err error, n int) {
		if err != nil {
			panic(err)
		} else {
			fmt.Println(string(buf))
		}
	})

	if err := ioc.RunPending(); err != nil {
		panic(err)
	}
}
