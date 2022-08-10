package main

import (
	"math/rand"
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

		go func() {
			for {
				time.Sleep(time.Duration(100+rand.Intn(500)) * time.Millisecond)
				conn.Write([]byte("hello"))
			}
		}()
	}
}
