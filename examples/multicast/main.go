package main

import (
	"flag"
	"fmt"
	"github.com/talostrading/sonic"
	"net"
)

// for a server: iperf -c 224.0.1.0 -u -i 1

var addr = flag.String("addr", "224.0.1.0:5001", "multicast group to join")

func main() {
	ioc := sonic.MustIO()
	defer ioc.Close()

	iff, err := net.InterfaceByName("en0")
	if err != nil {
		panic(err)
	}

	mc, err := sonic.NewUDPMulticastClient(ioc, iff, net.IPv4zero)
	if err != nil {
		panic(err)
	}

	udpAddr, err := net.ResolveUDPAddr("udp4", *addr)
	if err != nil {
		panic(err)
	}

	if err := mc.Join(udpAddr); err != nil {
		panic(err)
	}

	b := make([]byte, iff.MTU)
	var onRead sonic.AsyncReadCallbackPacket
	onRead = func(err error, n int, addr net.Addr) {
		if err != nil {
			panic(err)
		} else {
			b = b[:n]
			fmt.Println(string(b))
			b = b[:cap(b)]
			mc.AsyncReadFrom(b, onRead)
		}
	}
	mc.AsyncReadFrom(b, onRead)

	ioc.Run()
}
