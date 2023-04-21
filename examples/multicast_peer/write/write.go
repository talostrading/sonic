package main

import (
	"flag"
	"fmt"
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

	multicastIP, err := netip.ParseAddrPort(*multicastAddr)
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

	log.Printf("peer online local_addr=%s", peer.LocalAddr())

	b := []byte("hello, sonic!")
	n, err := peer.Write(b, multicastIP)
	fmt.Println(n, err)

	ioc.Run()
}
