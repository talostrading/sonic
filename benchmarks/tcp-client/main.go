package main

import (
	"flag"
	"net"
	"runtime/debug"
)

var n = flag.Int("n", 10, "number of connections")

func run(conn net.Conn) {
	b := make([]byte, 128)
	for {
		n, err := conn.Read(b)
		if err != nil {
			panic(err)
		}
		if n != 128 {
			panic("not 128")
		}
		n, err = conn.Write(b)
		if err != nil {
			panic(err)
		}
		if n != 128 {
			panic("not 128")
		}
	}
}

func main() {
	flag.Parse()
	debug.SetGCPercent(-1)
	for i := 0; i < *n; i++ {
		conn, err := net.Dial("tcp", "localhost:1234")
		if err != nil {
			panic(err)
		}
		go run(conn)
	}
	for {

	}
}
