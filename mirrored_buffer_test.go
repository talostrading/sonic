package sonic

import (
	"fmt"
	"log"
	"math/rand"
	"runtime"
	"syscall"
	"testing"
	"time"

	"github.com/talostrading/sonic/util"
)

func TestMirroredBuffer(t *testing.T) {
	size := syscall.Getpagesize()

	buf, err := NewMirroredBuffer(size, false)
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

	buf.Claim(1)
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

func TestMirroredBufferRandom(t *testing.T) {
	buf, err := NewMirroredBuffer(syscall.Getpagesize(), true)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := buf.Destroy(); err != nil {
			t.Fatalf("could not destroy buffer err=%v", err)
		}
	}()

	var (
		wrapped = 0
		rand    = rand.New(rand.NewSource(time.Now().UnixNano()))
		k       = 0
	)

	for wrapped == 0 || k < 1024*128 {
		n := rand.Intn(syscall.Getpagesize())
		b := buf.Claim(n)
		for i := range b {
			b[i] = byte(k % 127)
		}
		buf.Commit(n)
		if buf.Full() {

		} else {
			if buf.head > buf.tail {
				wrapped++
			}
		}
		buf.Consume(n)
		k++
	}
	if wrapped == 0 {
		t.Fatal("should have wrapped")
	}
}

func TestMirroredBufferGC(t *testing.T) {
	size := syscall.Getpagesize()

	buf, err := NewMirroredBuffer(size, false)
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

func TestMirroredBufferMaxSize(t *testing.T) {
	var (
		pageSize = syscall.Getpagesize()
		err      error
		size     = pageSize
		k        = 1
		buf      *MirroredBuffer
	)
	for err == nil && k < 1024*512 {
		buf, err = NewMirroredBuffer(size*k, false)
		buf.Destroy()
		k++
	}
	if err != nil {
		log.Printf(
			"allocated %d pages of size=%d (%s) then err=%v",
			k,
			size,
			err,
			int64(k*size),
		)
	} else {
		log.Printf(
			"allocated %d pages of size=%d (%s)",
			k,
			size,
			util.ByteCountSI(int64(k*size)),
		)
	}
}

func BenchmarkMirroredBuffer(b *testing.B) {
	var sizes []int
	for i := 1; i <= 16; i += 4 {
		sizes = append(sizes, syscall.Getpagesize()*i)
	}
	sizes = append(
		sizes,
		syscall.Getpagesize()*128,
		syscall.Getpagesize()*256,
		syscall.Getpagesize()*512,
	)

	for _, n := range sizes {
		n := n
		b.Run(
			fmt.Sprintf("byte_buffer_%s", util.ByteCountSI(int64(n))),
			func(b *testing.B) {
				buf := NewByteBuffer()
				buf.Reserve(n)

				letters := []byte("abcdefghijklmnopqrstuvwxyz")
				for i := 0; i < b.N; i++ {
					buf.Claim(func(b []byte) int {
						return copy(b, letters[:])
					})
					buf.Commit(7)
					buf.Consume(7)
				}
				b.ReportAllocs()
			})
	}

	for _, n := range sizes {
		n := n
		b.Run(
			fmt.Sprintf("mirrored_buffer_%s", util.ByteCountSI(int64(n))),
			func(b *testing.B) {
				buf, err := NewMirroredBuffer(n, false)
				if err != nil {
					b.Fatal(err)
				}

				letters := []byte("abcdefghijklmnopqrstuvwxyz")
				for i := 0; i < b.N; i++ {
					b := buf.Claim(7)
					copy(b, letters[:])
					buf.Commit(7)
					buf.Consume(7)
				}
				b.ReportAllocs()
				buf.Destroy()
			})
	}

}
