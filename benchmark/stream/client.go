package main

import (
	"flag"
	"log"
	"time"

	"net/http"
	_ "net/http/pprof"

	"github.com/HdrHistogram/hdrhistogram-go"
	"github.com/talostrading/sonic"
	"github.com/talostrading/sonic/sonicerrors"
	"github.com/talostrading/sonic/stream"
)

var (
	addr    = flag.String("addr", "localhost:8080", "address")
	pprof   = flag.String("pprof", "", "address for pprof; if empty, no pprof")
	verbose = flag.Bool("verbose", false, "if true, reads are printed")
	measure = flag.Int(
		"measure",
		0,
		"if > 0, batch size for latency measurements",
	)

	b [128]byte
)

func main() {
	flag.Parse()

	if *pprof != "" {
		go func() {
			if err := http.ListenAndServe(*pprof, nil); err != nil {
				log.Fatal(err)
			}
		}()
	}

	ioc := sonic.MustIO()
	defer ioc.Close()

	stream, err := stream.Connect(ioc, "tcp", *addr)
	if err != nil {
		log.Fatal(err)
	}
	defer stream.Close()

	var onRead func(error, int)
	onRead = func(err error, n int) {
		if err != nil {
			log.Fatal(err)
		}
		if *verbose {
			log.Println(string(b[:n]))
		}
		stream.AsyncRead(b[:], onRead)
	}
	stream.AsyncRead(b[:], onRead)

	if *measure > 0 {
		log.Printf("measuring loop latency batch_size=%d", *measure)

		hist := hdrhistogram.New(1, 1_000_000_000, 1)

		for {
			start := time.Now()
			n, err := ioc.PollOne()
			if err != nil && err != sonicerrors.ErrTimeout {
				log.Fatal(err)
			}
			if n > 0 {
				hist.RecordValue(time.Since(start).Microseconds())
				if hist.TotalCount() > int64(*measure) {
					log.Printf(
						"min/avg/max/stddev = %d/%d/%d/%dus",
						hist.Min(),
						int(hist.Mean()),
						hist.Max(),
						int(hist.StdDev()),
					)
					hist.Reset()
				}
			}
		}
	} else {
		for {
			ioc.PollOne()
		}
	}
}
