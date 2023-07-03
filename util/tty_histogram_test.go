package util

import (
	"io"
	"os"
	"testing"
	"time"
)

func TestTtyHistogram(t *testing.T) {
	hist := NewTtyHist(TtyHistOpts{
		Name:      "sample",
		Scale:     "ns",
		N:         8,
		MinPct:    0.1,
		Min:       1,
		Max:       100,
		Precision: 1,
		Writer:    os.Stdout,
	})

	hist.Add(1, 1, 1, 1)
	hist.Add(2, 2)
	hist.Add(3)
	hist.Add(4)
}

func BenchmarkTtyHistogram(b *testing.B) {
	hist := NewTtyHist(TtyHistOpts{
		Name:      "sample",
		Scale:     "ns",
		N:         4096,
		MinPct:    0.1,
		Min:       1,
		Max:       1_000_000_000,
		Precision: 1,
		Writer:    io.Discard,
	})

	last := time.Now()
	for i := 0; i < b.N; i++ {
		now := time.Now()
		hist.Add(now.Sub(last).Nanoseconds() + int64(i%100))
		last = now
	}
}
