package util

import (
	"fmt"
	"math"
)

type TrackerStats struct {
	Min    float64
	Avg    float64
	Max    float64
	StdDev float64
	Var    float64
}

func (s *TrackerStats) String() string {
	return fmt.Sprintf(
		"min/avg/max/stddev/var = %f/%f/%f/%f/%f",
		s.Min, s.Avg, s.Max, s.StdDev, s.Var)
}

func (s *TrackerStats) Reset() {
	s.Min = math.MaxFloat64
	s.Avg = 0.0
	s.Max = 0.0
	s.StdDev = 0.0
	s.Var = 0.0
}

type Tracker struct {
	stats   *TrackerStats
	N       int
	samples []int64
	ix      int
}

func NewTracker() *Tracker {
	return NewTrackerWithSamples(128)
}

func NewTrackerWithSamples(n int) *Tracker {
	return &Tracker{
		N:       n,
		samples: make([]int64, n),
		stats:   &TrackerStats{},
	}
}

func (t *Tracker) Record(sample int64) *TrackerStats {
	if t.ix >= len(t.samples) {
		t.stats.Reset()

		mean, stddev, variance := 0.0, 0.0, 0.0

		for _, s := range t.samples {
			sf := float64(s)

			if sf < t.stats.Min {
				t.stats.Min = sf
			}
			if sf > t.stats.Max {
				t.stats.Max = sf
			}
			mean += sf
		}
		mean /= float64(len(t.samples))

		for _, s := range t.samples {
			diff := float64(s) - mean
			stddev += diff * diff
		}
		stddev /= float64(len(t.samples))
		variance = stddev
		stddev = math.Sqrt(stddev)

		t.stats.Avg = mean
		t.stats.StdDev = stddev
		t.stats.Var = variance

		t.ix = 0
		return t.stats
	}

	t.samples[t.ix] = sample
	t.ix++

	return nil
}
