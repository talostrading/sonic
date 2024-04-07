package main

import (
	"flag"
	"net"
	"runtime/debug"
	"sync"
)

var n = flag.Int("n", 10, "number of connections")
var locked = flag.Bool("lck", false, "")
var rb = make([]byte, 128)
var wb = make([]byte, 128)
var rlck, wlck sync.Mutex

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

func run_lck(conn net.Conn) {
	for {
		rlck.Lock()
		n, err := conn.Read(rb)
		rlck.Unlock()
		if err != nil {
			panic(err)
		}
		if n != 128 {
			panic("not 128")
		}
		wlck.Lock()
		n, err = conn.Write(wb)
		wlck.Unlock()
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
		if *locked {
			go run_lck(conn)
		} else {
			go run(conn)
		}
	}
	for {

	}
}
