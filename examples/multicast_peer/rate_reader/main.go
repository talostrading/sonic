package main

import (
	"flag"
	"github.com/HdrHistogram/hdrhistogram-go"
	"github.com/talostrading/sonic"
	"github.com/talostrading/sonic/multicast"
	"github.com/talostrading/sonic/sonicerrors"
	"github.com/talostrading/sonic/util"
	"log"
	"net/netip"
	"time"
)

var (
	peerAddr = flag.String("p", ":5001",
		"Address to bound the peer to.")
	multicastIP = flag.String("m", "224.0.1.0",
		"multicast address to join")
	busy     = flag.Bool("busy", false, "if true, busy-wait for events")
	nSamples = flag.Int64("n", 128, "latency histogram samples, only relevant if busy == true")
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

	if err := peer.Join(*multicastIP); err != nil {
		panic(err)
	}

	log.Printf("joined multicast address")

	nBytes := 0
	b := make([]byte, 128)
	var onRead func(error, int, netip.AddrPort)
	onRead = func(err error, n int, addr netip.AddrPort) {
		if err != nil {
			panic(err)
		} else {
			b = b[:n]
			nBytes += n
			b = b[:cap(b)]
			peer.AsyncRead(b, onRead)
		}
	}
	peer.AsyncRead(b, onRead)

	log.Printf("starting event loop busy=%v", *busy)
	if *busy {
		hist := hdrhistogram.New(0, 10_00_000_000, 1)

		for {
			start := time.Now()

			n, err := ioc.PollOne()
			if err != nil && err != sonicerrors.ErrTimeout {
				panic(err)
			}
			if n > 0 {
				hist.RecordValue(time.Now().Sub(start).Nanoseconds())
				if hist.TotalCount() >= *nSamples {
					log.Printf("-------------------------------------------------------------------------------")

					log.Printf(
						"loop latency min/avg/max/stddev = %d/%d/%d/%d p50=%d p99=%d p99.9=%d",
						hist.Min(),
						int64(hist.Mean()),
						hist.Max(),
						int64(hist.StdDev()),
						hist.ValueAtPercentile(50.0),
						hist.ValueAtPercentile(99.0),
						hist.ValueAtPercentile(99.9),
					)
					hist.Reset()

					totalAsync := peer.Stats().AsyncImmediateReads + peer.Stats().AsyncScheduledReads
					pollRate := float64(peer.Stats().AsyncScheduledReads) / float64(totalAsync) * 100.0
					log.Printf(
						"async stats immediate=%d scheduled=%d total=%d poll_rate=%.2f%%",
						peer.Stats().AsyncImmediateReads,
						peer.Stats().AsyncScheduledReads,
						totalAsync,
						pollRate,
					)
					peer.Stats().Reset()

					log.Printf("read %s", util.ByteCountSI(int64(nBytes)))
					nBytes = 0

				}
			}
		}
	} else {
		ioc.Run()
	}
}
