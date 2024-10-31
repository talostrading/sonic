package main

import (
	"fmt"

	"github.com/csdenboer/sonic"
)

func main() {
	ioc := sonic.MustIO()
	defer ioc.Close()

	conn, err := sonic.Dial(ioc, "udp", "localhost:8080")
	if err != nil {
		panic(err)
	}

	fmt.Println(conn.LocalAddr())
}
