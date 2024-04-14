package util

import (
	"testing"
)

func TestIsPowerOfTwo(t *testing.T) {
	if IsPowerOfTwo(0) {
		t.Fatal("0 is not a power of two")
	}
	for i := 65; i < 128; i++ {
		if IsPowerOfTwo(i) {
			t.Fatalf("%d is not a power of two", i)
		}
	}
	for i := 0; i < 10; i++ {
		if !IsPowerOfTwo(1 << i) {
			t.Fatalf("%d is a power of two", 1<<i)
		}
		if !IsPowerOfTwo(-(1 << i)) {
			t.Fatalf("%d is a power of two", -(1 << i))
		}
	}

}
