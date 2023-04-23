package main

import (
	"encoding/binary"
	"flag"
	"github.com/HdrHistogram/hdrhistogram-go"
	"github.com/talostrading/sonic"
	"github.com/talostrading/sonic/multicast"
	"github.com/talostrading/sonic/sonicerrors"
	"github.com/talostrading/sonic/util"
	"log"
	"net/http"
	"net/netip"
	"time"

	"github.com/felixge/fgprof"
	_ "net/http/pprof"
)

var (
	peerAddr = flag.String("p", ":5001",
		"Address to bound the peer to.")
	multicastIP = flag.String("m", "224.0.1.0",
		"multicast address to join")
	busy     = flag.Bool("busy", false, "if true, busy-wait for events")
	nSamples = flag.Int64("n", 128, "latency histogram samples, only relevant if busy == true")
	profile  = flag.Bool("profile", false, "")
)

func main() {
	flag.Parse()

	if *profile {
		http.DefaultServeMux.Handle("/debug/fgprof", fgprof.Handler())
		go func() {
			log.Println(http.ListenAndServe(":6060", nil))
		}()
		go func() {
			log.Println(http.ListenAndServe("localhost:6061", nil))
		}()
	}

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

	var expectedSeq uint64 = 0
	nBytes := 0
	b := make([]byte, 128)
	var onRead func(error, int, netip.AddrPort)
	onRead = func(err error, n int, addr netip.AddrPort) {
		if err != nil {
			panic(err)
		} else {
			b = b[:n]

			seq := binary.BigEndian.Uint64(b)
			if expectedSeq == 0 {
				// handles reader starting after writer
				expectedSeq = seq + 1
			} else if seq != expectedSeq {
				// either writer restarted or a packet got lost
				log.Printf("expected seq=%d but got seq=%d", expectedSeq, seq)
				expectedSeq = seq + 1
			} else {
				// all good
				expectedSeq++
			}

			nBytes += n

			b = b[:cap(b)]
			peer.AsyncRead(b, onRead)
		}
	}
	peer.AsyncRead(b, onRead)

	log.Printf("starting event loop busy=%v", *busy)
	nPolled := 0
	if *busy {
		hist := hdrhistogram.New(0, 10_00_000_000, 1)

		for {
			start := time.Now()

			n, err := ioc.PollOne()
			if err != nil && err != sonicerrors.ErrTimeout {
				panic(err)
			}
			if n > 0 {
				nPolled += n
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

					log.Printf("async read perf=%.2f", peer.Stats().AsyncReadPerf())
					peer.Stats().Reset()
					nPolled = 0

					log.Printf("read %s", util.ByteCountSI(int64(nBytes)))
					nBytes = 0
				}
			}
		}
	} else {
		ioc.Run()
	}
}
