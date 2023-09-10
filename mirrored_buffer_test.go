package sonic

import (
	"syscall"
	"testing"
)

func TestMirroredBuffer(t *testing.T) {
	_, err := NewMirroredBuffer(syscall.Getpagesize())
	if err != nil {
		t.Fatal(err)
	}
}
