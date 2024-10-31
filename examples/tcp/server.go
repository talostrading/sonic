package main

import (
	"fmt"

	"github.com/csdenboer/sonic"
)

func main() {
	ioc := sonic.MustIO()
	defer ioc.Close()

	ln, err := sonic.Listen(ioc, "tcp", "localhost:8080")
	if err != nil {
		panic(err)
	}

	for {
		conn, err := ln.Accept()
		if err != nil {
			panic(err)
		}
		fmt.Println("accepted client", conn.RemoteAddr())

		_, err = conn.Write([]byte("hello, sonic!"))
		if err != nil {
			panic(err)
		}
	}
}
