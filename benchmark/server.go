package main

import (
	"flag"
	"log"
	"net"
	"time"
)

var addr = flag.String("addr", "localhost:8080", "address")

func main() {
	flag.Parse()

	ln, err := net.Listen("tcp", *addr)
	if err != nil {
		log.Fatal(err)
	}
	defer ln.Close()

	conn, err := ln.Accept()
	if err != nil {
		log.Fatal(err)
	}

	b := []byte("hello")

	for {
		_, err = conn.Write(b)
		if err != nil {
			log.Fatal(err)
		}

		time.Sleep(time.Millisecond)
	}
}
