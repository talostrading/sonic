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
	multicastAddr = flag.String("m", "224.0.1.0:5001",
		"multicast address to join")
)

func main() {
	flag.Parse()

	addr, err := netip.ParseAddrPort(*multicastAddr)
	if err != nil {
		panic(err)
	}

	ioc := sonic.MustIO()
	defer ioc.Close()

	peer, err := multicast.NewUDPPeer(ioc, "udp", *peerAddr)
	if err != nil {
		panic(err)
	}
	defer peer.Close()

	log.Printf(
		"peer online local_addr=%s sending to multicast=%s",
		peer.LocalAddr(),
		addr)

	b := []byte("hello, world!")

	var onWrite func(error, int)
	onWrite = func(err error, n int) {
		if err != nil {
			panic(err)
		} else {
			peer.AsyncWrite(b, addr, onWrite)
		}
	}
	peer.AsyncWrite(b, addr, onWrite)

	for {
		if err := ioc.RunOne(); err != nil {
			panic(err)
		}
	}
}
