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
				time.Sleep(time.Duration(1+rand.Intn(5)) * time.Second)
				conn.Write([]byte("hello"))
			}
		}()
	}
}
