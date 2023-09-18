package bytes

import (
	"fmt"
	"log"
	"math/rand"
	"runtime"
	"syscall"
	"testing"
	"time"

	"github.com/talostrading/sonic"
	"github.com/talostrading/sonic/util"
)

func TestMirroredBufferInit(t *testing.T) {
	pageSize := syscall.Getpagesize()

	buf, err := NewMirroredBuffer(pageSize, true)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := buf.Destroy(); err != nil {
			t.Fatal(err)
		}
	}()

	if buf.slice == nil {
		t.Fatal("backing slice should not nil")
	}
	if buf.baseAddr == nil {
		t.Fatal("backing slice's base address should not nil")
	}
	if buf.Size() != pageSize {
		t.Fatal("invalid size")
	}
	if buf.size != pageSize {
		t.Fatal("invalid size")
	}
	if buf.head != 0 {
		t.Fatal("head should be zero")
	}
	if buf.tail != 0 {
		t.Fatal("tail should be zero")
	}
	if buf.used != 0 {
		t.Fatal("used should be zero")
	}
	if len(buf.slice) != 2*buf.Size() {
		t.Fatal("backing slice should be twice the desired buffer size")
	}
}

func TestMirroredBufferInitSize1(t *testing.T) {
	pageSize := syscall.Getpagesize()

	buf, err := NewMirroredBuffer(pageSize, true)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := buf.Destroy(); err != nil {
			t.Fatal(err)
		}
	}()

	if buf.Size() != pageSize {
		t.Fatal("invalid size")
	}
}

func TestMirroredBufferInitSize2(t *testing.T) {
	pageSize := syscall.Getpagesize()

	buf, err := NewMirroredBuffer(1, true)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := buf.Destroy(); err != nil {
			t.Fatal(err)
		}
	}()

	if buf.Size() != pageSize {
		t.Fatal("invalid size")
	}
}

func TestMirroredBufferInitSize3(t *testing.T) {
	pageSize := syscall.Getpagesize()

	buf, err := NewMirroredBuffer(pageSize+1, true)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := buf.Destroy(); err != nil {
			t.Fatal(err)
		}
	}()

	if buf.Size() != pageSize*2 {
		t.Fatal("invalid size")
	}
}

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

	for i := 0; i < 2*size; i++ {
		if buf.slice[i] != 0 {
			t.Fatal("buffer should be zeroed")
		}
	}

	for i := 0; i < size; i++ {
		buf.slice[i] = 42
	}

	if buf.slice[0] != buf.slice[size] {
		t.Fatal("buffer should mirror the first byte")
	}

	for i := 0; i < size; i++ {
		if buf.slice[i] != buf.slice[size+i] {
			t.Fatal("buffer is not mirrored")
		}
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
		if b[0] != expectedValue-1 {
			t.Fatal("invalid head")
		}
	}

	if err := buf.Destroy(); err != nil {
		t.Fatal(err)
	}
}

func TestMirroredBuffer3(t *testing.T) {
	buf, err := NewMirroredBuffer(syscall.Getpagesize(), true)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := buf.Destroy(); err != nil {
			t.Fatalf("could not destroy buffer err=%v", err)
		}
	}()

	n := buf.Size() - 1
	b := buf.Claim(n)
	if len(b) != n {
		t.Fatal("invalid claim")
	}
	for i := range b {
		b[i] = 1
	}
	if buf.Commit(n) != n {
		t.Fatal("invalid commit")
	}

	if buf.Consume(10) != 10 {
		t.Fatal("invalid consume")
	}

	// This write will cross the mirror: 1 byte taken from the end, 5 from the
	// beginning.
	b = buf.Claim(6)
	if len(b) != 6 {
		t.Fatal("invalid claim")
	}
	for i := range b {
		b[i] = 2
	}
	if buf.Commit(6) != 6 {
		t.Fatal("invalid commit")
	}

	b = buf.Head()
	if b[len(b)-1] != 2 {
		t.Fatal("wrong tail")
	}
	for i := 0; i < 5; i++ {
		if b[i] != 2 {
			t.Fatal("wrong head")
		}
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

		if !buf.Full() && buf.head > buf.tail {
			wrapped++
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

func TestMirroredBufferMultiple(t *testing.T) {
	var buffers []*MirroredBuffer
	for i := 0; i < 8; i++ {
		buf, err := NewMirroredBuffer(syscall.Getpagesize(), true)
		if err != nil {
			t.Fatal(err)
		}
		buffers = append(buffers, buf)
	}

	for k, buf := range buffers {
		b := buf.Claim(8)
		for i := 0; i < 8; i++ {
			b[i] = byte(k + 1)
		}
		buf.Commit(8)
	}

	for k, buf := range buffers {
		b := buf.Head()
		for i := 0; i < 8; i++ {
			if b[i] != byte(k+1) {
				t.Fatal("invalid buffer")
			}
		}
	}

	for _, buf := range buffers {
		if err := buf.Destroy(); err != nil {
			t.Fatal(err)
		}
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
					buf := sonic.NewByteBuffer()
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
