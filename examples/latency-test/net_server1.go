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
	rw   = flag.Bool("rw", true, "if true, each connection will handle the read and write ends in separate goroutines")

	// If true, then we use less memory but there will be quite some contention
	// on the histogram. We want to test for both cases.
	shareHist  = flag.Bool("sh", false, "if true, then all connections share the same histogram")
	globalHist = hdrhistogram.New(1, 10_000_000, 1)
	lck        sync.Mutex

	connID = 1
)

func DecodeNanos(from []byte) int64 {
	return int64(binary.LittleEndian.Uint64(from))
}

func EncodeNanos(into []byte) {
	binary.LittleEndian.PutUint64(into, uint64(util.GetMonoNanos()))
}

func Record(id int, hist *hdrhistogram.Histogram, diff int64) {
	if err := hist.RecordValue(diff); err != nil {
		// diff might be to big for the histogram and we ignore it
		log.Printf("err=%v\n", err)
	}
	if hist.TotalCount() >= *n {
		log.Printf(
			"conn_id=%d min/avg/max/stddev = %d/%d/%d/%d p50=%d p75=%d p90=%d p95=%d p99=%d p99.5=%d p99.9=%d",
			id,
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
	defer conn.Close()

	id := connID
	connID++

	log.Printf(
		"accepted connection local_addr=%s remote_addr=%s\n",
		conn.LocalAddr(), conn.RemoteAddr())

	var localHist *hdrhistogram.Histogram
	if !*shareHist {
		log.Printf("conn %d using local histogram\n", id)
		localHist = hdrhistogram.New(1, 10_000_000, 1)
	} else {
		log.Printf("conn %d using global histogram\n", id)
	}

	if *rw {
		var wg sync.WaitGroup
		wg.Add(2)

		go func() {
			log.Printf("conn %d started writer goroutine\n", id)
			b := make([]byte, 8)
			for {
				EncodeNanos(b)
				n, err := conn.Write(b)
				if n != 8 || err != nil {
					panic(err)
				}
			}
		}()
		go func() {
			log.Printf("conn %d started reader goroutine\n", id)
			b := make([]byte, 8)
			for {
				n, err := conn.Read(b)
				if n != 8 || err != nil {
					panic(err)
				}

				diff := util.GetMonoNanos() - DecodeNanos(b)
				if *shareHist {
					lck.Lock()
					Record(id, globalHist, diff)
					lck.Unlock()
				} else {
					Record(id, localHist, diff)
				}
			}
		}()

		wg.Wait()
	} else {
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

			diff := util.GetMonoNanos() - DecodeNanos(b)
			if *shareHist {
				lck.Lock()
				Record(id, globalHist, diff)
				lck.Unlock()
			} else {
				Record(id, localHist, diff)
			}
		}
	}
}

func main() {
	flag.Parse()

	log.Printf("listening on %s\n", *addr)

	ln, err := net.Listen("tcp", *addr)
	if err != nil {
		panic(err)
	}
	defer ln.Close()

	for {
		conn, err := ln.Accept()
		if err != nil {
			panic(err)
		}
		go handle(conn)
	}
}
