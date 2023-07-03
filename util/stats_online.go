package util

import "math"

// OnlineStats gives you min/avg/max/stddev in O(1) time and space.
//
// Note that stddev is less accurate than the one reported by Stats.
type OnlineStats struct {
	res *Result

	n      int
	meanSq float64
}

func NewOnlineStats() *OnlineStats {
	res := &Result{}
	res.Reset()
	return &OnlineStats{res: res}
}

func (s *OnlineStats) Add(xs ...float64) {
	for _, x := range xs {
		if x > s.res.Max {
			s.res.Max = x
		}

		if x < s.res.Min {
			s.res.Min = x
		}

		s.n++
		delta := x - s.res.Avg
		s.res.Avg += delta / float64(s.n)
		s.meanSq += delta * (x - s.res.Avg)
	}

	if s.n >= 2 {
		s.res.StdDev = math.Sqrt(s.meanSq / float64(s.n-1))
	}
}

func (s *OnlineStats) Result() *Result {
	return s.res
}

func (s *OnlineStats) Reset() {
	s.n = 0
	s.meanSq = 0
	s.res.Reset()
}

func (s *OnlineStats) Len() int {
	return s.n
}
