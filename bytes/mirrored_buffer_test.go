package bytes

import (
	"fmt"
	"io"
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

	log.Printf("mirrored buffer is in %s", buf.Name())

	if buf.slice == nil {
		t.Fatal("backing slice should not nil")
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
		b = buf.slice[:buf.size]
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

	b = buf.slice[:buf.size]
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

func TestMirroredBufferSizes(t *testing.T) {
	var (
		pageSize = syscall.Getpagesize()
		err      error
		size     = pageSize
		k        = 1
		buf      *MirroredBuffer
	)

	log.Printf("pagesize=%d", pageSize)

	for err == nil && k < 1024*512 && size*k < 1024*1024*1024 /* 1GB */ {
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

func TestMirroredBufferHugeSize(t *testing.T) {
	// We should be good mapping huge amounts of memory due to on-demand paging
	// on Linux/BSD. As long as we don't prefault or write to the entire
	// buffer... Still you should not make a buffer this big in production - the
	// system will go OOM.
	var (
		size     = 1024 * 1024 * 1024 * 1024 // 1TB
		pageSize = syscall.Getpagesize()
	)
	size = (size / pageSize) * pageSize
	buf, err := NewMirroredBuffer(size, false)
	if err != nil {
		t.Fatal(err)
	}
	defer buf.Destroy()

	// this allocates on page
	b := buf.Claim(128)
	for i := range b {
		b[i] = 128
	}
	b[0] = 128
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
		b := buf.slice[:buf.size]
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

func TestMirroredBufferMmapBehaviour(t *testing.T) {
	// This test shows the behaviour of MAP_PRIVATE and MAP_SHARED. In short,
	// pages mmapped with MAP_PRIVATE are subject to copy on write. This is best
	// showcased through an example:
	// - we make two MAP_PRIVATE mappings of a file,
	// - initially both of these mappings point to the same memory locations
	// - we change the values of some bytes in the first private mapping. The
	// kernel does copy on write here. It first copies the pages of memory that
	// refer to the changed values, and then changes the values in the copies.
	// - this leaves the second mapping unchaged.
	// - copy on write is efficient: only the changed pages are copied
	//
	// There is no copy-on-write for MAP_SHARED. If we make two MAP_SHARED
	// mappings and change the memory in one of them, the change is reflected in
	// the other one immediately. They both point to the same pages at all
	// times.

	size := syscall.Getpagesize()

	name := "/tmp/sonic_mirrored_buffer_test"
	fd, err := syscall.Open(
		name,
		syscall.O_CREAT|syscall.O_RDWR,  // file is readable/writeable
		syscall.S_IRUSR|syscall.S_IWUSR, // user can read/write to this file
	)
	if err != nil {
		t.Fatal(err)
	}
	defer syscall.Close(fd)
	if err := syscall.Truncate(name, int64(size)); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Unlink(name); err != nil {
		t.Fatal(err)
	}

	mmap := func(flags int) []byte {
		b, err := syscall.Mmap(
			fd,
			0,
			size,
			syscall.PROT_READ|syscall.PROT_WRITE,
			flags,
		)
		if err != nil {
			t.Fatal(err)
		}
		return b
	}

	var (
		private1 = mmap(syscall.MAP_PRIVATE)[:4]
		private2 = mmap(syscall.MAP_PRIVATE)[:4]
		shared1  = mmap(syscall.MAP_SHARED)[:4]
		shared2  = mmap(syscall.MAP_SHARED)[:4]
	)
	defer syscall.Munmap(private1)
	defer syscall.Munmap(private2)
	defer syscall.Munmap(shared1)
	defer syscall.Munmap(shared2)

	for i := 0; i < 4; i++ {
		private1[i] = 0
		private2[i] = 0
		shared1[i] = 0
		shared2[i] = 0
	}

	log.Printf(
		"%v %v %v %v :: init",
		private1,
		private2,
		shared1,
		shared2,
	)

	private1[0] = 16
	if private2[0] == 16 || shared1[0] == 16 || shared2[0] == 16 {
		t.Fatal("invalid mapping")
	}

	log.Printf(
		"%v %v %v %v :: private1[0]=16 ",
		private1,
		private2,
		shared1,
		shared2,
	)

	private2[0] = 32
	if private1[0] == 32 || shared1[0] == 32 || shared2[0] == 32 {
		t.Fatal("invalid mapping")
	}

	log.Printf(
		"%v %v %v %v :: private2[0]=32 ",
		private1,
		private2,
		shared1,
		shared2,
	)

	shared1[0] = 64
	if private1[0] == 64 || private2[0] == 64 || shared2[0] != 64 {
		t.Fatal("invalid mapping")
	}

	log.Printf(
		"%v %v %v %v :: shared1[0]=64 ",
		private1,
		private2,
		shared1,
		shared2,
	)
}

func BenchmarkMirroredBuffer(b *testing.B) {
	// This benchmarks a typical workflow for a stream transport buffer. What
	// usually happens is:
	// - a part of the buffer is claimed (here we claim the whole buffer)
	// - we read from the network in the claimed part. Here that is replaced
	//   with a memcpy
	// - the buffer now contains multiple messages that can be decoded
	// - a decode means: commit some bytes, decode them into an internal
	//   type and the consume them.

	var (
		toCopy = make([]byte, 1024*128) // what we copy in the claimed part

		// The number of messages to decode after a Claim+memcpy
		nMessages = []int{1, 2, 4, 8, 16, 32, 64, 128, 256, 512}
	)

	for _, nMessage := range nMessages {
		nMessage := nMessage
		bytesPerMessage := len(toCopy) / nMessage
		b.Run(
			fmt.Sprintf(
				"byte_buffer_%s_%d",
				util.ByteCountSI(int64(len(toCopy))),
				nMessage,
			),
			func(b *testing.B) {
				{
					buf := sonic.NewByteBuffer()
					buf.Reserve(len(toCopy))
					buf.Prefault()

					b.ResetTimer()

					sum := 0
					for i := 0; i < b.N; i++ {
						buf.Claim(func(b []byte) int {
							return copy(b, toCopy)
						})
						for buf.WriteLen() > 0 {
							buf.Commit(bytesPerMessage)
							// here we would decode the bytes into an internal
							// type
							buf.Consume(bytesPerMessage)
						}
					}
					b.ReportAllocs()
					fmt.Fprint(io.Discard, sum)
				}
				runtime.GC()
			})
	}

	for _, chunk := range nMessages {
		chunk := chunk
		consume := len(toCopy) / chunk
		b.Run(
			fmt.Sprintf(
				"mirrored_buffer_%s_%d",
				util.ByteCountSI(int64(len(toCopy))),
				chunk,
			),
			func(b *testing.B) {
				{
					buf, err := NewMirroredBuffer(len(toCopy), true)
					if err != nil {
						b.Fatal(err)
					}
					defer buf.Destroy()
					buf.Prefault()

					b.ResetTimer()

					sum := 0
					for i := 0; i < b.N; i++ {
						b := buf.Claim(len(toCopy))
						copy(b, toCopy)
						for buf.UsedSpace() > 0 {
							buf.Commit(consume)
							// here we would decode the bytes into an internal
							// type
							buf.Consume(consume)
						}
					}
					fmt.Fprint(io.Discard, sum)
					b.ReportAllocs()
				}
				runtime.GC()
			})
	}
}
