package util

import (
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/HdrHistogram/hdrhistogram-go"
)

type TtyHistOpts struct {
	Name      string
	Scale     string
	N         int64
	MinPct    float64
	Min       int64
	Max       int64
	Precision int
	Writer    io.Writer
}

type TtyHist struct {
	opts TtyHistOpts

	hdr  *hdrhistogram.Histogram
	tabw *tabwriter.Writer
	n    int
}

func NewTtyHist(opts TtyHistOpts) *TtyHist {
	h := &TtyHist{
		opts: opts,
		hdr:  hdrhistogram.New(opts.Min, opts.Max, opts.Precision),
		tabw: tabwriter.NewWriter(opts.Writer, 2, 2, 2, byte(' '), 0),
	}
	return h
}

func (h *TtyHist) Add(xs ...int64) {
	for _, x := range xs {
		_ = h.hdr.RecordValue(x)
	}
	if h.hdr.TotalCount() >= h.opts.N {
		h.n++
		h.report()
		h.hdr.Reset()
	}
}

func (h *TtyHist) Reported() int {
	return h.n
}

func (h *TtyHist) report() {
	if h.opts.Writer == nil {
		return
	}

	fmt.Fprint(h.opts.Writer,
		"----------------------------------------------\n")
	fmt.Fprint(h.opts.Writer,
		"----------------------------------------------\n")

	fmt.Fprintf(h.opts.Writer,
		"%v histogram report=%d name=%s samples=%d scale=%s\n\n",
		time.Now().Format("2006-01-02 15:04:05"),
		h.n, h.opts.Name, h.opts.N, h.opts.Scale,
	)
	fmt.Fprintf(h.opts.Writer,
		"summary min/avg/max/stddev = %d/%.3f/%d/%.3f %s\n",
		h.hdr.Min(), h.hdr.Mean(), h.hdr.Max(), h.hdr.StdDev(), h.opts.Scale)
	fmt.Fprintf(h.opts.Writer,
		"50th percentile=%d %s\n", h.hdr.ValueAtPercentile(50.0), h.opts.Scale)
	fmt.Fprintf(h.opts.Writer,
		"75th percentile=%d %s\n", h.hdr.ValueAtPercentile(75.0), h.opts.Scale)
	fmt.Fprintf(h.opts.Writer,
		"90th percentile=%d %s\n", h.hdr.ValueAtPercentile(90.0), h.opts.Scale)
	fmt.Fprintf(h.opts.Writer,
		"95th percentile=%d %s\n", h.hdr.ValueAtPercentile(95.0), h.opts.Scale)
	fmt.Fprintf(h.opts.Writer,
		"99th percentile=%d %s\n", h.hdr.ValueAtPercentile(99.0), h.opts.Scale)

	var minBinCount, maxBinCount int64 = math.MaxInt64, math.MinInt64
	for _, bin := range h.hdr.Distribution() {
		pct := float64(bin.Count) * 100.0 / float64(h.hdr.TotalCount())
		if pct < h.opts.MinPct {
			continue
		}

		if bin.Count < minBinCount {
			minBinCount = bin.Count
		}

		if bin.Count > maxBinCount {
			maxBinCount = bin.Count
		}
	}

	yFmt := func(y int64) string {
		if y > 0 {
			return strconv.FormatInt(y, 10)
		}
		return ""
	}

	for _, bin := range h.hdr.Distribution() {
		pct := float64(bin.Count) * 100.0 / float64(h.hdr.TotalCount())
		if pct < h.opts.MinPct {
			continue
		}

		barSize := 0
		if h.hdr.Min() != h.hdr.Max() {
			fraction := float64(bin.Count-minBinCount) /
				float64(maxBinCount-minBinCount)

			barSize = int(math.Ceil(fraction * 10))
		}
		if barSize == 0 {
			barSize = 1
		}

		from := bin.From
		to := bin.To
		if from == to {
			to += 1
		}

		fmt.Fprintf(h.tabw,
			"%d-%d %s\t%.3g%%\t%s\n",
			from, to,
			h.opts.Scale,
			pct,
			strings.Repeat("|", int(barSize))+"\t"+yFmt(bin.Count),
		)
	}

	_ = h.tabw.Flush()
}
