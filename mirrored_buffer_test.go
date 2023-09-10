package sonic

import (
	"runtime"
	"syscall"
	"testing"
)

func TestMirroredBuffer1(t *testing.T) {
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

	if err := buf.Destroy(); err != nil {
		t.Fatal("buffer should be destroyed")
	}
}

func TestMirroredBuffer2(t *testing.T) {
	size := syscall.Getpagesize()

	buf, err := NewMirroredBuffer(size)
	if err != nil {
		t.Fatal(err)
	}

	for k := 0; k < size/7*4; k++ {
		runtime.GC()

		buf.Claim(7)
		buf.Commit(7)
		buf.Consume(7)
	}

	var memstats runtime.MemStats
	runtime.ReadMemStats(&memstats)
	if memstats.NumGC != uint32(size/7*4) {
		t.Fatal("did not GC")
	}

	if buf.UsedSpace() != 0 {
		t.Fatal("buffer should be empty")
	}

	b := buf.Claim(128)
	for i := 0; i < 128; i++ {
		b[i] = 42
	}
	buf.Commit(128)

	if err := buf.Destroy(); err != nil {
		t.Fatal("buffer should be destroyed")
	}
}
