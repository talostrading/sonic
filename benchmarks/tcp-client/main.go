package main

import (
	"flag"
	"net"
	"runtime/debug"
	"sync"
)

var n = flag.Int("n", 10, "number of connections")
var b = make([]byte, 128)
var lck sync.Mutex

func run(conn net.Conn) {
	for {
		lck.Lock()
		n, err := conn.Read(b)
		lck.Unlock()
		if err != nil {
			panic(err)
		}
		if n != 128 {
			panic("not 128")
		}
		lck.Lock()
		n, err = conn.Write(b)
		lck.Unlock()
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
