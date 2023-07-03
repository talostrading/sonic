package util

import "math"

type Result struct {
	Min    float64
	Max    float64
	Avg    float64
	StdDev float64
}

func (r *Result) Reset() {
	r.Min = math.MaxFloat64
	r.Max = -math.MaxFloat64
	r.Avg = 0.0
	r.StdDev = 0.0
}

// Stats gives you min/avg/max/stddev in O(1) time O(n) space.
type Stats struct {
	xs  []float64
	res *Result
	n   int
	cb  func(*Result)
}

func NewStats(n int, cb func(*Result)) *Stats {
	return &Stats{
		xs:  make([]float64, 0, n),
		res: &Result{},
		n:   n,
		cb:  cb,
	}
}

func (s *Stats) Add(xs ...float64) {
	s.xs = append(s.xs, xs...)

	if s.cb != nil && len(s.xs) >= s.n {
		s.cb(s.res)
		s.Reset()
	}
}

func (s *Stats) Reset() {
	s.xs = s.xs[:0]
}

func (s *Stats) Result() *Result {
	n := len(s.xs)

	s.res.Min = math.MaxFloat64
	s.res.Max = -math.MaxFloat64

	s.res.Avg = 0.0
	for i := 0; i < n; i++ {
		if s.xs[i] > s.res.Max {
			s.res.Max = s.xs[i]
		}

		if s.xs[i] < s.res.Min {
			s.res.Min = s.xs[i]
		}

		s.res.Avg += s.xs[i]
	}
	s.res.Avg /= float64(n)

	s.res.StdDev = 0.0
	for i := 0; i < n; i++ {
		diff := (s.xs[i] - s.res.Avg)
		s.res.StdDev += diff * diff
	}
	s.res.StdDev /= float64(n)
	s.res.StdDev = math.Sqrt(s.res.StdDev)

	return s.res
}

func (s *Stats) Len() int {
	return len(s.xs)
}
