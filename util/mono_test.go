package util

import (
	"fmt"
	"math/rand"
	"testing"
	"time"
)

func BenchmarkGetMonoNanos(b *testing.B) {
	var s int64 = 0
	for i := 0; i < b.N; i++ {
		s += GetMonoNanos()
	}
	b.ReportAllocs()
	fmt.Println(s)
}

func BenchmarkTimeNow(b *testing.B) {
	var s int64 = 0
	for i := 0; i < b.N; i++ {
		s += time.Now().UnixNano()
	}
	b.ReportAllocs()
	fmt.Println(s)
}

func TestMono(t *testing.T) {
	for i := 0; i < 100; i++ {
		startMono := GetMonoNanos()

		sleepingFor := time.Duration(rand.Intn(100_000)) * time.Microsecond

		start := time.Now()
		var diff time.Duration
		for {
			diff = time.Now().Sub(start)
			if diff > sleepingFor {
				break
			}
		}

		diffMono := GetMonoNanos() - startMono
		if diffMono < sleepingFor.Nanoseconds() {
			t.Fatalf(
				"did not sleep long enough sleeping_for=%s diff=%d",
				sleepingFor, diff)
		}

		delta := diffMono - diff.Nanoseconds()
		if delta < 0 {
			delta = -delta
		}

		if delta > 100_000 { // 100 micros
			t.Fatalf("mono and time.Now() difference not the same diff=%d", delta)
		}
	}
}
