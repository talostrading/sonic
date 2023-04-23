package main

import (
	"encoding/binary"
	"flag"
	"log"

	"github.com/HdrHistogram/hdrhistogram-go"
	"github.com/talostrading/sonic"
	"github.com/talostrading/sonic/util"
)

var (
	addr = flag.String("addr", "localhost:8080", "address to connect to")
	n    = flag.Int64("n", 1024*32, "samples in a batch")

	hist = hdrhistogram.New(1, 10_000_000, 1)
)

func DecodeNanos(from []byte) int64 {
	return int64(binary.LittleEndian.Uint64(from))
}

func EncodeNanos(into []byte) {
	binary.LittleEndian.PutUint64(into, uint64(util.GetMonoTimeNanos()))
}

func Record(diff int64) {
	if err := hist.RecordValue(diff); err != nil {
		// diff might be to big for the histogram and we ignore it
		log.Printf("err=%v\n", err)
		return
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

func main() {
	flag.Parse()

	ioc := sonic.MustIO()
	defer ioc.Close()

	conn, err := sonic.Dial(ioc, "tcp", *addr)
	if err != nil {
		panic(err)
	}

	b := make([]byte, 8)

	var (
		onRead  sonic.AsyncCallback
		onWrite sonic.AsyncCallback
	)

	onWrite = func(err error, _ int) {
		if err != nil {
			panic(err)
		}
		conn.AsyncRead(b, onRead)
	}

	onRead = func(err error, _ int) {
		if err != nil {
			panic(err)
		} else {
			Record(util.GetMonoTimeNanos() - DecodeNanos(b))
			EncodeNanos(b)
			conn.AsyncWrite(b, onWrite)
		}
	}

	EncodeNanos(b)
	conn.AsyncWrite(b, onWrite)

	for {
		ioc.PollOne()
	}
}
