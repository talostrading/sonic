package main

import (
	"fmt"
	"github.com/talostrading/sonic"
	"github.com/talostrading/sonic/sonicopts"
	"time"
)

func main() {
	ioc := sonic.MustIO()
	defer ioc.Close()

	conn, err := sonic.DialReconnecting(
		ioc,
		"tcp",
		"localhost:8080",
		sonicopts.Nonblocking(false),
		sonicopts.UseNetConn(false))
	if err != nil {
		panic(err)
	}

	b := make([]byte, 128)
	for {
		ioc.PollOne()

		b = b[:cap(b)]
		n, err := conn.Read(b)
		if err != nil {
			continue
		}
		b = b[:n]
		fmt.Println(time.Now(), "read", string(b))
	}
}
