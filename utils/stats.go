package utils

import (
	"fmt"
	"math"
)

type Stats struct {
	name  string
	xs    []float64
	ready bool

	mean     float64
	stddev   float64
	variance float64

	csv    bool
	header bool
}

func NewStats(name string, csv bool, samples int) *Stats {
	return &Stats{
		xs:     make([]float64, 0, samples),
		name:   name,
		csv:    csv,
		header: true,
	}
}

func (s *Stats) Add(xs ...float64) {
	if len(s.xs) >= cap(s.xs) {
		s.Report()
		s.Reset()
	}

	s.ready = false
	s.xs = append(s.xs, xs...)
}

func (s *Stats) Mean() float64 {
	s.calculate()
	return s.mean

}

func (s *Stats) Var() float64 {
	s.calculate()
	return s.variance
}

func (s *Stats) StdDev() float64 {
	s.calculate()
	return s.stddev
}

func (s *Stats) Report() {
	if s.csv {
		if s.header {
			fmt.Println("name,mean,stddev,samples")
			s.header = false
		} else {
			fmt.Printf("%s,%.5f,%.5f,%d\n", s.name, s.Mean(), s.StdDev(), len(s.xs))
		}
	} else {
		fmt.Printf("stats name=%s mean=%.5f stddev=%.5f samples=%d\n", s.name, s.Mean(), s.StdDev(), len(s.xs))
	}
}

func (s *Stats) Reset() {
	s.xs = s.xs[:0]
}

func (s *Stats) calculate() {
	if s.ready == true {
		return
	}
	s.ready = true

	nf := float64(len(s.xs))

	s.mean = 0.0
	for _, x := range s.xs {
		s.mean += x
	}
	s.mean /= nf

	s.variance = 0.0
	for _, x := range s.xs {
		t := (x - s.mean)
		s.variance += t * t
	}
	s.variance /= float64(len(s.xs))

	s.stddev = math.Sqrt(s.variance)
}
