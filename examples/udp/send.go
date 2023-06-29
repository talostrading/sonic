package main

import (
	"fmt"
	"net"

	"github.com/talostrading/sonic"
)

func main() {
	ioc := sonic.MustIO()
	defer ioc.Close()

	// Senders do not need an address.
	conn, err := sonic.NewPacketConn(ioc, "udp", "")
	if err != nil {
		panic(err)
	}

	toAddr, err := net.ResolveUDPAddr("udp", "localhost:8080")
	if err != nil {
		panic(err)
	}

	i := 0
	exit := false
	var onWrite sonic.AsyncWriteCallbackPacket
	onWrite = func(err error) {
		if err != nil {
			panic(err)
		}
		i++
		if i < 100 {
			conn.AsyncWriteTo([]byte("hello"), toAddr, onWrite)
		} else {
			exit = true
		}
	}
	conn.AsyncWriteTo([]byte("hello"), toAddr, onWrite)

	for !exit {
		ioc.Run()
	}

	fmt.Printf("sent %d packets successfully, exiting...\n", i)
}
