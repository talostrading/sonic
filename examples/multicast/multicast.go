package main

import (
	"flag"
	"fmt"
	"github.com/talostrading/sonic"
	"log"
	"net"
	"strings"
)

var (
	iffName = flag.String("if", "en0", "interface to handle multicast traffic")
	addrs   = flag.String("addrs", "224.0.1.0:40000", "multicast group addresses (comma separated) to join, from 224.0.1.0 till 239.255.255.255. Can also be IPv6")
)

func main() {
	flag.Parse()

	iff, err := net.InterfaceByName(*iffName)
	if err != nil {
		panic(err)
	}

	var multicastAddrs []*net.UDPAddr
	for _, addr := range strings.Split(*addrs, ",") {
		multicastAddr, err := net.ResolveUDPAddr("udp", addr)
		if err != nil {
			panic(err)
		}
		multicastAddrs = append(multicastAddrs, multicastAddr)
	}

	ioc := sonic.MustIO()
	defer ioc.Close()

	client, err := sonic.NewMulticastClient(ioc, iff, net.IPv4zero)
	if err != nil {
		panic(err)
	}

	for _, addr := range multicastAddrs {
		if err := client.Join(addr); err != nil {
			panic(err)
		}
		log.Printf("joined multicast group %s, local_addr=%s", addr, client.LocalAddr())
	}

	b := make([]byte, 128)
	var onRead sonic.AsyncReadCallbackPacket
	onRead = func(err error, n int, addr net.Addr) {
		if err != nil {
			panic(err)
		} else {
			b = b[:n]
			fmt.Println(string(b), addr)
			b = b[:cap(b)]
			client.AsyncReadFrom(b, onRead)
		}
	}
	client.AsyncReadFrom(b, onRead)

	ioc.Run()
}
