package util

import (
	"fmt"
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
