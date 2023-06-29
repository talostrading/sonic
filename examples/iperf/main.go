package main

/*
Simple bandwidth test for sonic TCP.

Client is iperf with the command below, server is this sonic program.

$ iperf -c 127.0.0.1 -e -i 5 -l 8KB
The default port is 5001, as set in the flags below.
This makes iperf write 8KB chunks to the sonic server.

The bandwidth results vary greatly with the values set for -l and *n flag below.
*/

import (
	"flag"
	"log"

	"github.com/talostrading/sonic"
	"github.com/talostrading/sonic/internal"
)

var (
	addr = flag.String("addr", "127.0.0.1:5001", "iperf server address")
	n    = flag.Int("n", 8192, "read buffer size")
)

func main() {
	flag.Parse()

	ioc := sonic.MustIO()
	defer ioc.Close()

	ln, err := sonic.Listen(ioc, "tcp", *addr)
	if err != nil {
		panic(err)
	}

	b := make([]byte, *n)
	var onAccept sonic.AcceptCallback
	onAccept = func(err error, conn sonic.Conn) {
		if err != nil {
			log.Fatalf("could not accept err=%v", err)
		} else {
			ln.AsyncAccept(onAccept)
			log.Printf("accepted addr=%s", conn.RemoteAddr())

			nonblocking, err := internal.IsNonblocking(conn.RawFd())
			if err != nil {
				log.Fatal(err)
				return
			}

			noDelay, err := internal.IsNoDelay(conn.RawFd())
			if err != nil {
				log.Fatal(err)
				return
			}

			log.Printf("socket is_nonblocking=%v is_nodelay=%v", nonblocking, noDelay)

			var onRead sonic.AsyncCallback
			onRead = func(err error, n int) {
				if err != nil {
					panic(err)
				}
				b = b[:n]
				b = b[:cap(b)]
				conn.AsyncRead(b, onRead)
			}
			conn.AsyncRead(b, onRead)
		}
	}
	ln.AsyncAccept(onAccept)

	for {
		ioc.PollOne()
	}
}
