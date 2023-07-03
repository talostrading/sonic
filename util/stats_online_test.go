package util

import (
	"math"
	"math/rand"
	"testing"
)

const epsilon = 0.001

func equal(a, b, eps float64) bool {
	return math.Abs(a-b) < eps
}

func TestOnlineStats1(t *testing.T) {
	s := NewOnlineStats()

	check := func() {
		if !equal(s.res.Avg, 2.5, epsilon) {
			t.Fatal("wrong average")
		}
		if !equal(s.res.Min, 1.0, epsilon) {
			t.Fatal("wrong min")
		}
		if !equal(s.res.Max, 4.0, epsilon) {
			t.Fatal("wrong max")
		}
		if !equal(s.res.StdDev, 1.29, 0.1) {
			t.Fatal("wrong stddev")
		}
		if s.Len() != 4 {
			t.Fatal("wrong n")
		}
	}

	s.Add(1.0, 2.0, 3.0, 4.0)
	check()

	s.Reset()
	s.Add(1.0, 2.0, 3.0, 4.0)
	check()
}

func TestOnlineStats2(t *testing.T) {
	online := NewOnlineStats()
	offline := NewStats(100, nil)

	for i := 0; i < 100; i++ {
		num := rand.Float64() * 1000
		online.Add(num)
		offline.Add(num)
	}

	var (
		given    float64
		expected float64
	)

	given = online.Result().Avg
	expected = offline.Result().Avg
	if !equal(given, expected, epsilon) {
		t.Fatalf("wrong average given=%v expected=%v", given, expected)
	}

	given = online.Result().Min
	expected = offline.Result().Min
	if !equal(given, expected, epsilon) {
		t.Fatalf("wrong min given=%v expected=%v", given, expected)
	}

	given = online.Result().Max
	expected = offline.Result().Max
	if !equal(given, expected, epsilon) {
		t.Fatalf("wrong max given=%v expected=%v", given, expected)
	}

	given = online.Result().StdDev
	expected = offline.Result().StdDev
	if !equal(given, expected, 10) {
		t.Fatalf("wrong stddev given=%v expected=%v", given, expected)
	}
}
