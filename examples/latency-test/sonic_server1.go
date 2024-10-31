package main

import (
	"encoding/binary"
	"flag"
	"log"

	"github.com/HdrHistogram/hdrhistogram-go"
	"github.com/csdenboer/sonic"
	"github.com/csdenboer/sonic/sonicopts"
	"github.com/csdenboer/sonic/util"
)

var (
	addr = flag.String("addr", "localhost:8080", "server address")
	n    = flag.Int64("n", 1024*32, "samples in a batch")
	hot  = flag.Bool("hot", true, "if true, busy-wait for events")

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

	ln, err := sonic.Listen(
		ioc,
		"tcp",
		*addr,
		sonicopts.Nonblocking(true))
	if err != nil {
		panic(err)
	}
	defer ln.Close()

	var (
		b = make([]byte, 8)

		onAccept sonic.AcceptCallback
	)

	onAccept = func(err error, conn sonic.Conn) {
		log.Printf(
			"accepted connection local_addr=%s remote_addr=%s\n",
			conn.LocalAddr(), conn.RemoteAddr())

		ln.AsyncAccept(onAccept)

		var (
			onWrite sonic.AsyncCallback
			onRead  sonic.AsyncCallback
		)

		onRead = func(err error, n int) {
			if err != nil {
				conn.Close()
				panic(err)
			} else {
				Record(util.GetMonoTimeNanos() - DecodeNanos(b))

				EncodeNanos(b)
				conn.AsyncWrite(b, onWrite)
			}
		}

		onWrite = func(err error, n int) {
			if err != nil {
				conn.Close()
				panic(err)
			} else {
				conn.AsyncRead(b, onRead)
			}
		}

		EncodeNanos(b)
		conn.AsyncWrite(b, onWrite)
	}

	ln.AsyncAccept(onAccept)

	log.Printf("listening on %s\n", *addr)

	if *hot {
		log.Print("busy wait")
		for {
			ioc.PollOne()
		}
	} else {
		ioc.Run()
	}
}
