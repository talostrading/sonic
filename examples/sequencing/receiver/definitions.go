package main

import (
	"log"

	"github.com/HdrHistogram/hdrhistogram-go"
	"github.com/talostrading/sonic"
)

type Reader interface {
	Setup()
}

type ByteBufferProcessor interface {
	Process(seq int, payload []byte, b *sonic.ByteBuffer) int
	Buffered() int
}

func PrintHistogram(hist *hdrhistogram.Histogram, buffered int) {
	log.Printf(
		"process latency min/avg/max/stddev = %d/%d/%d/%dus p95=%d p99=%d p99.5=%d p99.9=%d p99.99=%d n_buffered=%d",
		int(hist.Min()),
		int(hist.Mean()),
		int(hist.Max()),
		int(hist.StdDev()),
		hist.ValueAtPercentile(95.0),
		hist.ValueAtPercentile(99.0),
		hist.ValueAtPercentile(99.5),
		hist.ValueAtPercentile(99.9),
		hist.ValueAtPercentile(99.99),
		buffered,
	)
}
