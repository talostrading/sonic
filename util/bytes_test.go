package util

import (
	"testing"
)

func TestExtendByteSlice(t *testing.T) {
	b := make([]byte, 10)
	b = ExtendByteSlice(b, 20)
	l, c := len(b), cap(b)
	if l != 20 && c < 20 {
		t.Fatalf("invalid len=%d cap=%d; need len=%d cap>=%d", l, c, 20, 20)
	}
}
