package main

import (
	"encoding/binary"
	"flag"
	"fmt"
	"log"
	"net/netip"
	"time"

	"github.com/HdrHistogram/hdrhistogram-go"
	"github.com/talostrading/sonic"
	"github.com/talostrading/sonic/multicast"
	"github.com/talostrading/sonic/util"
)

var (
	addr        = flag.String("addr", "224.0.0.224:8080", "multicast address on which to send")
	ifaceName   = flag.String("interface", "", "interface to join on")
	bufsize     = flag.Int("bufsize", 1024*1024, "buffer size")
	readbufsize = flag.Int("readbufsize", 1024, "read buffer size, usually some multiple of MTU")
	debug       = flag.Bool("debug", false, "debug")
	batchSize   = flag.Int64("batchsize", 128, "histogram batch size")

	letters = []byte("abcdefghijklmnopqrstuvwxyz")
)

func main() {
	flag.Parse()

	ioc := sonic.MustIO()
	defer ioc.Close()

	paddr, err := netip.ParseAddrPort(*addr)
	if err != nil {
		panic(err)
	}
	ip := paddr.Addr().String()

	peer, err := multicast.NewUDPPeer(ioc, "udp4", *addr)
	if err != nil {
		panic(err)
	}
	log.Printf("udp peer bound to %s", *addr)

	if *ifaceName != "" {
		err = peer.JoinOn(multicast.IP(ip), multicast.InterfaceName(*ifaceName))
		if err != nil {
			panic(err)
		}
		log.Printf("joined %s on %s", ip, *ifaceName)
	} else {
		err = peer.Join(multicast.IP(ip))
		if err != nil {
			panic(err)
		}
		log.Printf("joined %s", ip)
	}

	var (
		expected uint32 = 1
	)

	process := func(b []byte) {
		seq := binary.BigEndian.Uint32(b)
		if expected != seq {
			panic(fmt.Errorf("expected=%d got=%d", expected, seq))
		}
		expected++

		n := binary.BigEndian.Uint32(b[4:])
		payload := b[8:]
		if len(payload) != int(n) {
			panic("actual payload length does not match encoded payload length")
		}

		for i := 0; i < len(payload); i++ {
			if payload[i] != letters[i%len(letters)] {
				panic(fmt.Errorf(
					"wrong payload=%s", string(payload),
				))
			}
		}

		if *debug {
			log.Printf("seq=%d n=%d payload=%s", seq, n, string(payload))
		}
	}

	log.Printf("buffer_size=%s", util.ByteCountSI(int64(*bufsize)))
	buf := sonic.NewBipBuffer(*bufsize)

	hist := hdrhistogram.New(1, 10_000_000, 1)

	var onRead func(error, int, netip.AddrPort)
	onRead = func(err error, n int, _ netip.AddrPort) {
		if err != nil {
			panic(err)
		}

		start := time.Now()
		{
			buf.Commit(n)
			process(buf.Data()[:n])
			buf.Consume(n)
		}
		diff := time.Since(start).Microseconds()
		hist.RecordValue(diff)

		if hist.TotalCount() >= *batchSize {
			hist.Reset()
		} else {
			log.Printf(
				"min/avg/max/stddev = %d/%d/%d/%dus p95=%d p99=%d p99.9=%d",
				hist.Min(),
				int64(hist.Mean()),
				hist.Max(),
				int64(hist.StdDev()),
				hist.ValueAtPercentile(95.0),
				hist.ValueAtPercentile(99.0),
				hist.ValueAtPercentile(99.9),
			)
		}

		if b := buf.Claim(*readbufsize); b != nil {
			peer.AsyncRead(b, onRead)
		} else {
			panic("buffer is full")
		}
	}

	peer.AsyncRead(buf.Claim(*readbufsize), onRead)

	for {
		_, _ = ioc.PollOne()
	}
}
