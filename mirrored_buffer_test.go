package sonic

import (
	"syscall"
	"testing"
)

func TestMirroredBuffer(t *testing.T) {
	size := syscall.Getpagesize()
	buf, err := NewMirroredBuffer(size)
	if err != nil {
		t.Fatal(err)
	}

	b := buf.Claim(size)
	if b == nil {
		t.Fatal("should claim")
	}
}
