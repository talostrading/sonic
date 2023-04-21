package main

import (
	"flag"
	"github.com/talostrading/sonic"
	"github.com/talostrading/sonic/multicast"
	"log"
	"net/netip"
)

var (
	peerAddr = flag.String("p", ":5001",
		"Address to bound the peer to.")
	multicastAddr = flag.String("m", "224.0.1.0",
		"multicast address to join")
)

func main() {
	flag.Parse()

	ioc := sonic.MustIO()
	defer ioc.Close()

	peer, err := multicast.NewUDPPeer(ioc, "udp", *peerAddr)
	if err != nil {
		panic(err)
	}
	defer peer.Close()

	log.Printf("peer online local_addr=%s", peer.LocalAddr())

	if err := peer.Join(*multicastAddr); err != nil {
		panic(err)
	}

	log.Printf("joined multicast address")

	b := make([]byte, 128)
	var onRead func(error, int, netip.AddrPort)
	onRead = func(err error, n int, addr netip.AddrPort) {
		if err != nil {
			panic(err)
		} else {
			b = b[:n]
			log.Printf("received %d bytes from %s", n, addr)
			b = b[:cap(b)]
			peer.AsyncRead(b, onRead)
		}
	}
	peer.AsyncRead(b, onRead)

	ioc.Run()
}
