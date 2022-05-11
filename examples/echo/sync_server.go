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
	fmt.Println("this is printed only after accepting something, as we block on accept")

	_, err = conn.Write([]byte("hello, sonic!"))
	if err != nil {
		panic(err)
	}
}
