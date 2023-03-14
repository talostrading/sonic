package main

import (
	"encoding/binary"
	"flag"
	"log"
	"net"
	"sync"

	"github.com/HdrHistogram/hdrhistogram-go"
	"github.com/talostrading/sonic/util"
)

var (
	addr = flag.String("addr", "localhost:8080", "server address")
	n    = flag.Int64("n", 1024*32, "samples in a batch")

	hist = hdrhistogram.New(1, 10_000_000, 1)
	lck  sync.Mutex
)

func DecodeNanos(from []byte) int64 {
	return int64(binary.LittleEndian.Uint64(from))
}

func EncodeNanos(into []byte) {
	binary.LittleEndian.PutUint64(into, uint64(util.GetMonoNanos()))
}

func Record(diff int64) {
	lck.Lock()
	defer lck.Unlock()

	if err := hist.RecordValue(diff); err != nil {
		panic(err)
	}
	if hist.TotalCount() >= *n {
		log.Printf(
			"min/avg/max/stddev = %d/%d/%d/%d p50=%d p75=%d p90=%d p95=%d p99=%d p99.5=%d p99.9=%d",
			hist.Min(),
			int64(hist.Mean()),
			hist.Max(),
			int64(hist.StdDev()),
			hist.ValueAtPercentile(50.0),
			hist.ValueAtPercentile(75.0),
			hist.ValueAtPercentile(90.0),
			hist.ValueAtPercentile(95.0),
			hist.ValueAtPercentile(99.0),
			hist.ValueAtPercentile(99.5),
			hist.ValueAtPercentile(99.9),
		)
		hist.Reset()
	}
}

func handle(conn net.Conn) {
	log.Printf(
		"accepted connection local_addr=%s remote_addr=%s\n",
		conn.LocalAddr(), conn.RemoteAddr())

	b := make([]byte, 8)

	for {
		EncodeNanos(b)
		n, err := conn.Write(b)
		if n != 8 || err != nil {
			panic(err)
		}

		n, err = conn.Read(b)
		if n != 8 || err != nil {
			panic(err)
		}

		Record(util.GetMonoNanos() - DecodeNanos(b))
	}
}

func main() {
	flag.Parse()

	log.Printf("listening on %s\n", *addr)

	ln, err := net.Listen("tcp", *addr)
	if err != nil {
		panic(err)
	}
	for {
		conn, err := ln.Accept()
		if err != nil {
			panic(err)
		}
		go handle(conn)
	}
}
