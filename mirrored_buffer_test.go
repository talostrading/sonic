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

func TestMirroredBuffer1(t *testing.T) {
	size := syscall.Getpagesize()
	buf, err := NewMirroredBuffer(size, true)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := buf.Destroy(); err != nil {
			t.Fatal(err)
		}
	}()

	buf.slice[0] = 42

	if buf.slice[0] != buf.slice[size] {
		t.Fatal("buffer should mirror the first byte")
	}
}

func TestMirroredBuffer2(t *testing.T) {
	size := syscall.Getpagesize()
	buf, err := NewMirroredBuffer(size, true)
	if err != nil {
		t.Fatal(err)
	}

	var (
		v     byte = 0
		chunk      = size / 64
		b     []byte
	)
	{
		b = buf.Claim(1)
		if len(b) != 1 {
			t.Fatal("should have claimed 1")
		}
		b[0] = v
		if buf.Commit(1) != 1 {
			t.Fatal("should have committed 1")
		}
		v++
	}
	{
		for {
			b = buf.Claim(chunk)
			if b == nil {
				t.Fatal("claimed slice should not be nil")
			}
			if len(b) != chunk {
				break
			}

			for i := range b {
				b[i] = v
			}
			buf.Commit(chunk)
			v++
		}
		if buf.FreeSpace() != chunk-1 {
			t.Fatal("wrong free space")
		}
		if buf.head > buf.tail {
			t.Fatal("buffer should not wrap")
		}
	}
	{
		if buf.Consume(1) != 1 {
			t.Fatal("should have consumed 1")
		}
		if buf.FreeSpace() != chunk {
			t.Fatalf("should have %d bytes available", chunk)
		}
	}
	{
		b = buf.Claim(chunk)
		if len(b) != chunk {
			t.Fatalf("should have claimed %d", chunk)
		}
		if buf.FreeSpace() != chunk {
			t.Fatalf("should have %d bytes available", chunk)
		}
		for i := range b {
			b[i] = v
		}
		if buf.Commit(chunk) != chunk {
			t.Fatalf("should have committed %d", chunk)
		}
		if buf.FreeSpace() != 0 {
			t.Fatal("should have no free space")
		}
		if buf.head != buf.tail {
			t.Fatal("buffer should be wrapped and full")
		}
	}
	{
		b = buf.Head()
		if len(b) == 0 {
			t.Fatal("should not be zero")
		}

		var (
			offset             = buf.head
			expectedValue byte = 1
		)
		for k := 0; k < size/chunk; k++ {
			slice := b[offset+k*chunk:]
			slice = slice[:chunk]
			for i := range slice {
				if slice[i] != expectedValue {
					t.Fatal("buffer is in wrong state")
				}
			}
			expectedValue++
		}
	}

	if err := buf.Destroy(); err != nil {
		t.Fatal(err)
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
	letters := []byte("abcdefghijklmnopqrstuvwxyz")

	for _, n := range sizes {
		n := n
		b.Run(
			fmt.Sprintf("byte_buffer_%s", util.ByteCountSI(int64(n))),
			func(b *testing.B) {
				{
					buf := NewByteBuffer()
					buf.Reserve(n)
					buf.Prefault()

					b.ResetTimer()

					for i := 0; i < b.N; i++ {
						buf.Claim(func(b []byte) int {
							return copy(b, letters[:])
						})
						buf.Commit(7)
						buf.Consume(7)
					}
					b.ReportAllocs()
				}
				runtime.GC()
			})
	}

	for _, n := range sizes {
		n := n
		b.Run(
			fmt.Sprintf("mirrored_buffer_%s", util.ByteCountSI(int64(n))),
			func(b *testing.B) {
				{
					buf, err := NewMirroredBuffer(n, false)
					if err != nil {
						b.Fatal(err)
					}
					defer buf.Destroy()
					buf.Prefault()

					b.ResetTimer()

					for i := 0; i < b.N; i++ {
						b := buf.Claim(7)
						copy(b, letters[:])
						buf.Commit(7)
						buf.Consume(7)
					}
					b.ReportAllocs()
				}
				runtime.GC()
			})
	}
}
