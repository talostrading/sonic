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

	n := size/2 + 1
	b := buf.Claim(n)
	if len(b) != n {
		t.Fatal("wrong claim")
	}

	for i := range b {
		b[i] = 42
	}
	if buf.Commit(n) != n {
		t.Fatal("wrong commit")
	}

	if buf.Consume(n-1) != n-1 {
		t.Fatal("wrong consume")
	}
	if buf.UsedSpace() != 1 {
		t.Fatal("wrong used space")
	}

	if buf.head >= buf.tail {
		t.Fatal("buffer should not be wrapped")
	}

	// The next slice will cross the mirror boundary.
	n = buf.FreeSpace() - 1
	b = buf.Claim(n)
	if len(b) != n {
		t.Fatal("wrong claim")
	}
	for i := range b {
		b[i] = 84
	}

	if buf.Commit(n) != n {
		t.Fatal("wrong claim")
	}
	
	if buf.head <= buf.tail {
		t.Fatal("buffer should be wrapped")
	}

	if buf.FreeSpace() != 1 {
		t.Fatal("wrong free space")
	}
	if buf.Full() {
		t.Fatal("buffer should not be full")
	}

	b = buf.Claim(1)
	buf.Commit(1)

	if buf.FreeSpace() != 0 || buf.UsedSpace() != size {
		t.Fatal("wrong free/used space")
	}
	if !buf.Full() {
		t.Fatal("buffer should be full")
	}
}
