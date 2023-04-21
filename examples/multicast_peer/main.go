package main

import (
	"flag"
	"fmt"
	"github.com/talostrading/sonic/multicast"
	"github.com/talostrading/sonic/sonicerrors"
	"log"
)

var (
	peerAddr = flag.String("p", ":5001",
		"Address to bound the peer to.")
	multicastAddr = flag.String("m", "224.0.1.0",
		"multicast address to join")
)

func main() {
	flag.Parse()

	peer, err := multicast.NewUDPPeer("udp", *peerAddr)
	if err != nil {
		panic(err)
	}
	defer peer.Close()

	log.Printf("peer online local_addr=%s", peer.LocalAddr())

	if err := peer.Join(*multicastAddr); err != nil {
		panic(err)
	}

	log.Printf("joined multicast address", )

	b := make([]byte, 128)
	for {
		n, addr, err := peer.RecvFrom(b)
		if err != nil && err != sonicerrors.ErrWouldBlock {
			panic(err)
		} else if err == nil {
			fmt.Println(n, addr, string(b))
		}
	}
}
