package main

import (
	"flag"
	"fmt"
	"net"
	"runtime"
	"runtime/debug"
)

var (
	ngoroutine = flag.Int("ngoroutine", 0, "If positive, sets the maximum number of go-routines which should run at the same time")
)

func main() {
	flag.Parse()

	if *ngoroutine > 0 {
		runtime.GOMAXPROCS(*ngoroutine)
	}

	debug.SetGCPercent(-1)

	ln, err := net.Listen("tcp", "127.0.0.1:8080")
	if err != nil {
		panic(err)
	}

	id := 0
	for {
		conn, err := ln.Accept()
		if err != nil {
			panic(err)
		}

		id++
		fmt.Println("accepted", id)

		go func() {
			defer conn.Close()
			var buf [1024]byte

			for {
				n, err := conn.Read(buf[:])
				if err != nil {
					return
				}

				_, err = conn.Write(buf[:n])
				if err != nil {
					return
				}
			}
		}()
	}

}
