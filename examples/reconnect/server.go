package main

import (
	"net"
	"time"
)

func main() {
	ln, err := net.Listen("tcp", "localhost:8080")
	if err != nil {
		panic(err)
	}

	for {
		conn, err := ln.Accept()
		if err != nil {
			panic(err)
		}

		for i := 0; i < 5; i++ {
			time.Sleep(time.Millisecond)
			conn.Write([]byte("hello"))
		}

		conn.Close()
	}
}
