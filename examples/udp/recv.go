package main

import (
	"fmt"
	"net"

	"github.com/csdenboer/sonic"
)

func main() {
	ioc := sonic.MustIO()
	defer ioc.Close()

	// Receivers must have an address.
	conn, err := sonic.NewPacketConn(ioc, "udp", "localhost:8080")
	if err != nil {
		panic(err)
	}

	i := 0
	b := make([]byte, 128)
	var onRead sonic.AsyncReadCallbackPacket
	onRead = func(err error, n int, from net.Addr) {
		if err != nil {
			panic(err)
		}

		i++
		b = b[:n]
		fmt.Println(i, "received", string(b), "from", from)
		b = b[:cap(b)]
		conn.AsyncReadAllFrom(b, onRead)
	}
	conn.AsyncReadFrom(b, onRead)

	ioc.Run()
}
