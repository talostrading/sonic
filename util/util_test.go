package util

import (
	"testing"
)

func TestExtendByteSlice(t *testing.T) {
	b := make([]byte, 10)
	b = ExtendSlice(b, 20)
	l, c := len(b), cap(b)
	if l != 20 && c < 20 {
		t.Fatalf("invalid len=%d cap=%d; need len=%d cap>=%d", l, c, 20, 20)
	}
}

func TestCopyBytes(t *testing.T) {
	dst := make([]byte, 1)
	src := []byte("hello")

	dst = CopySlice(dst, src)
	if len(dst) != len(src) {
		t.Fatalf("wrong length expected=%d given=%d", len(src), len(dst))
	}

	dst = make([]byte, 10)
	dst = CopySlice(dst, src)
	if len(dst) != len(src) {
		t.Fatalf("wrong length expected=%d given=%d", len(src), len(dst))
	}
}
