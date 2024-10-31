package main

import (
	"encoding/binary"
	"flag"
	"log"
	"net/netip"
	"time"

	"github.com/csdenboer/sonic"
	"github.com/csdenboer/sonic/multicast"
	"github.com/csdenboer/sonic/util"
)

var (
	peerAddr = flag.String("p", ":5001",
		"Address to bound the peer to.")
	multicastAddr = flag.String("m", "224.0.1.0:5001",
		"multicast address to join")
	period      = flag.Duration("period", 10*time.Microsecond, "how much to wait in-between writes")
	payloadSize = flag.Int("payload", 32, "payload size in bytes")
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

	b := make([]byte, *payloadSize)

	start := time.Now()
	nBytes := 0
	var seq uint64 = 1
	for {
		binary.BigEndian.PutUint64(b, seq)
		n, err := peer.Write(b, addr)
		if err == nil {
			seq++

			nBytes += n
			if now := time.Now(); now.Sub(start).Seconds() >= 1 {
				start = now
				log.Printf("rate = %s/s", util.ByteCountSI(int64(nBytes)))
				nBytes = 0
			}
		}
		time.Sleep(*period)
	}
}
