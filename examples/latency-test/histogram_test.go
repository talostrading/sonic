package main

import (
	"encoding/binary"
	"io"
	"testing"

	"github.com/HdrHistogram/hdrhistogram-go"
)

// BenchmarkT* are timed with -benchtime as they might take a long time otherwise
// BenchmarkU* are untimed
// See Makefile.

func BenchmarkTHistogramRecord(b *testing.B) {
	hist := hdrhistogram.New(1, 10_000_000, 1)
	for i := 0; i < b.N; i++ {
		hist.RecordValue(int64(i))
	}
}

func BenchmarkUHistogramMin(b *testing.B) {
	hist := hdrhistogram.New(1, 10_000_000, 1)
	for i := 0; i < 10_000; i++ {
		hist.RecordValue(int64(i))
	}

	var v int64 = 0
	for i := 0; i < b.N; i++ {
		v += int64(hist.Min())
	}

	if v < 0 {
		v = 0
	}

	buf := make([]byte, 8)
	binary.LittleEndian.PutUint64(buf, uint64(v))
	io.Discard.Write(buf)
}

func BenchmarkUHistogramAvg(b *testing.B) {
	hist := hdrhistogram.New(1, 10_000_000, 1)
	for i := 0; i < 10_000; i++ {
		hist.RecordValue(int64(i))
	}

	var v int64 = 0
	for i := 0; i < b.N; i++ {
		v += int64(hist.Mean())
	}

	if v < 0 {
		v = 0
	}

	buf := make([]byte, 8)
	binary.LittleEndian.PutUint64(buf, uint64(v))
	io.Discard.Write(buf)
}

func BenchmarkUHistogramMax(b *testing.B) {
	hist := hdrhistogram.New(1, 10_000_000, 1)
	for i := 0; i < 10_000; i++ {
		hist.RecordValue(int64(i))
	}

	var v int64 = 0
	for i := 0; i < b.N; i++ {
		v += int64(hist.Max())
	}

	if v < 0 {
		v = 0
	}

	buf := make([]byte, 8)
	binary.LittleEndian.PutUint64(buf, uint64(v))
	io.Discard.Write(buf)
}

func BenchmarkUHistogramStdDev(b *testing.B) {
	hist := hdrhistogram.New(1, 10_000_000, 1)
	for i := 0; i < 10_000; i++ {
		hist.RecordValue(int64(i))
	}

	var v int64 = 0
	for i := 0; i < b.N; i++ {
		v += int64(hist.StdDev())
	}

	if v < 0 {
		v = 0
	}

	buf := make([]byte, 8)
	binary.LittleEndian.PutUint64(buf, uint64(v))
	io.Discard.Write(buf)
}

func BenchmarkUHistogramPercentile(b *testing.B) {
	hist := hdrhistogram.New(1, 10_000_000, 1)
	for i := 0; i < 10_000; i++ {
		hist.RecordValue(int64(i))
	}

	var v int64 = 0
	for i := 0; i < b.N; i++ {
		v += int64(hist.ValueAtPercentile(95.0))
	}

	if v < 0 {
		v = 0
	}

	buf := make([]byte, 8)
	binary.LittleEndian.PutUint64(buf, uint64(v))
	io.Discard.Write(buf)
}
