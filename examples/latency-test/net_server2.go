package main

import (
	"flag"
	"log"
	"net"
	"sync"
)

var (
	addr = flag.String("addr", "localhost:8080", "address to connect to")
	rw   = flag.Bool("rw", false, "if true, each connection will handle the read and write ends in separate goroutines")
)

func handle(conn net.Conn) {
	if *rw {
		var wg sync.WaitGroup
		wg.Add(2)

		b := make([]byte, 8)
		var lck sync.Mutex
		go func() {
			log.Print("started reader goroutine")
			for {
				lck.Lock()
				n, err := conn.Read(b)
				lck.Unlock()
				if n != 8 || err != nil {
					panic(err)
				}
			}
		}()
		go func() {
			log.Print("started writer goroutine")
			for {
				lck.Lock()
				n, err := conn.Write(b)
				lck.Unlock()
				if n != 8 || err != nil {
					panic(err)
				}
			}
		}()

		wg.Wait()
	} else {
		log.Printf("handling connection %s\n", conn.LocalAddr())
		b := make([]byte, 8)
		for {
			n, err := conn.Read(b)
			if n != 8 || err != nil {
				panic(err)
			}

			n, err = conn.Write(b)
			if n != 8 || err != nil {
				panic(err)
			}
		}
	}
}

func main() {
	flag.Parse()

	ln, err := net.Listen("tcp", *addr)
	if err != nil {
		panic(err)
	}

	for {
		conn, err := ln.Accept()
		if err != nil {
			panic(err)
		}
		go handle(conn)
	}
}
