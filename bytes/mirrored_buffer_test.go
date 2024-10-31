package bytes

import (
	"fmt"
	"io"
	"log"
	"math/rand"
	"runtime"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/csdenboer/sonic"
	"github.com/csdenboer/sonic/util"
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

func TestMirroredBufferSize(t *testing.T) {
	// We assert that a buffer's size is at least as larged as the passed size
	// and a multiple of page size.
	pageSize := syscall.Getpagesize()

	for _, size := range []int{1, pageSize, pageSize + 1} {
		buf, err := NewMirroredBuffer(size, false)
		if err != nil {
			t.Fatal(err)
		}
		defer func() {
			if err := buf.Destroy(); err != nil {
				t.Fatal(err)
			}
		}()

		if !(buf.Size() >= 1 && buf.Size()%pageSize == 0) {
			t.Fatalf("invalid size=%d", buf.Size())
		}
	}

	// We ensure callers cannot create a buffer of size 0.
	if _, err := NewMirroredBuffer(0, false); err == nil {
		t.Fatal("should not be able to create a buffer of size zero")
	}
}

func TestMirroredBufferEnsureMirroring(t *testing.T) {
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

	for i := 0; i < 2*pageSize; i++ {
		if buf.slice[i] != 0 {
			t.Fatal("buffer should be zeroed")
		}
	}

	// Set the left mapping to 42. This change should be reflected in the right
	// mapping.
	for i := 0; i < pageSize; i++ {
		buf.slice[i] = 42
	}

	for i := 0; i < pageSize; i++ {
		if buf.slice[i] != buf.slice[pageSize+i] {
			t.Fatal("buffer is not mirrored")
		}
	}
}

func TestMirroredBufferClaim(t *testing.T) {
	buf, err := NewMirroredBuffer(syscall.Getpagesize(), false)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := buf.Destroy(); err != nil {
			t.Fatal(err)
		}
	}()

	b := buf.Claim(buf.Size() - 1)
	for i := range b {
		b[i] = 1
	}
	buf.Commit(len(b))

	if len(buf.Claim(2)) != 1 {
		t.Fatal("invalid claim")
	}
}

func TestMirroredBufferWritesOnMirrorBoundary(t *testing.T) {
	buf, err := NewMirroredBuffer(syscall.Getpagesize(), true)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := buf.Destroy(); err != nil {
			t.Fatalf("could not destroy buffer err=%v", err)
		}
	}()

	// empty claim such that the next one can cross the mirror boundary
	b := buf.Claim(buf.Size() - 1)
	buf.Commit(len(b))
	buf.Consume(1)

	if !(buf.head == 1 && buf.tail == buf.Size()-1) {
		t.Fatal("invalid state")
	}

	// this claim crosses the mirror boundary
	b = buf.Claim(2)
	if len(b) != 2 {
		t.Fatal("invalid claim")
	}
	b[0], b[1] = 1, 1
	buf.Commit(2)

	// assert that we wrapped
	if !(buf.head == 1 && buf.tail == 1) {
		t.Fatal("invalid state")
	}

	// assert left part
	if !(buf.slice[0] == 1 && buf.slice[buf.size-1] == 1) {
		t.Fatal("invalid state")
	}
	for i := 1; i < buf.size-1; i++ {
		if buf.slice[i] != 0 {
			t.Fatal("invalid state")
		}
	}

	// assert right part
	if !(buf.slice[buf.size] == 1 && buf.slice[2*buf.size-1] == 1) {
		t.Fatal("invalid state")
	}
	for i := buf.size + 1; i < 2*buf.size-1; i++ {
		if buf.slice[i] != 0 {
			t.Fatal("invalid state")
		}
	}
}

func TestMirroredBufferRandomClaimCommitConsume(t *testing.T) {
	pageSize := syscall.Getpagesize()
	buf, err := NewMirroredBuffer(pageSize, true)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := buf.Destroy(); err != nil {
			t.Fatalf("could not destroy buffer err=%v", err)
		}
	}()

	var (
		rand      = rand.New(rand.NewSource(time.Now().UnixNano()))
		wrapCount = 0
		iteration = 0
	)

	for wrapCount == 0 || iteration < pageSize*128 {
		claim := rand.Intn(pageSize)
		b := buf.Claim(claim)
		for i := range b {
			b[i] = byte(iteration % 127)
		}
		buf.Commit(claim)

		if buf.head >= buf.tail && buf.used > 0 {
			wrapCount++
		}

		buf.Consume(claim)
		iteration++
	}
	if wrapCount == 0 {
		t.Fatal("should have wrapped")
	}
}

func TestMirroredBufferDoesNotGetGarbageCollected(t *testing.T) {
	pageSize := syscall.Getpagesize()
	buf, err := NewMirroredBuffer(pageSize, false)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := buf.Destroy(); err != nil {
			t.Fatalf("could not destroy buffer err=%v", err)
		}
	}()

	iterations := pageSize / 7 * 4
	for k := 0; k < iterations; k++ {
		runtime.GC()

		b := buf.Claim(7)
		for i := range b {
			b[i] = 42
		}
		buf.Commit(7)
		buf.Consume(7)
	}

	// ensure the garbage collector has run each iteration
	var memstats runtime.MemStats
	runtime.ReadMemStats(&memstats)
	if memstats.NumGC != uint32(iterations) {
		t.Fatal("did not GC")
	}

	if buf.UsedSpace() != 0 {
		t.Fatal("buffer should be empty")
	}

	for i := range buf.slice {
		if buf.slice[i] != 42 {
			t.Fatal("invalid state")
		}
	}
}

func TestMirroredBufferAllocateDifferentSizes(t *testing.T) {
	var (
		pageSize = syscall.Getpagesize()
		err      error
		pages    = 1
		buf      *MirroredBuffer
	)

	const (
		MaxSize  = 1024 * 1024 * 1024 // 1GB
		MaxPages = 1024 * 512
	)

	log.Printf("page_size=%d", pageSize)

	for err == nil && pages < MaxPages && pageSize*pages < MaxSize {
		buf, err = NewMirroredBuffer(pageSize*pages, false)
		buf.Destroy()
		pages++
	}

	if err != nil {
		log.Printf(
			"could not allocate a buffer of size=%s page_size=%d pages=%d err=%v",
			util.ByteCountSI(int64(pages*pageSize)),
			pageSize,
			pages,
			err,
		)
	} else {
		log.Printf(
			"largest buffer allocated was of size=%s page_size=%d pages=%d",
			util.ByteCountSI(int64(pages*pageSize)),
			pageSize,
			pages,
		)
	}
}

func TestMirroredBufferAllocateHugeSize(t *testing.T) {
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

func TestMirroredBufferAllocateMultipleBuffersConcurrently(t *testing.T) {
	var (
		buffers []*MirroredBuffer
		values  []byte
		lck     sync.Mutex
		wg      sync.WaitGroup
	)

	const NBuffers = 64

	for k := 0; k < NBuffers; k++ {
		value := byte(k)
		wg.Add(1)
		go func() {
			defer wg.Done()

			// increase the chance of concurrent allocation
			time.Sleep(time.Millisecond)

			buf, err := NewMirroredBuffer(syscall.Getpagesize(), true)
			if err != nil {
				t.Error(err)
			}

			b := buf.Claim(buf.Size())
			for i := range b {
				b[i] = value
			}
			buf.Commit(buf.Size())

			lck.Lock()
			defer lck.Unlock()
			buffers = append(buffers, buf)
			values = append(values, value)
		}()
	}

	wg.Wait()

	for k := 0; k < NBuffers; k++ {
		b := buffers[k].slice
		for i := range b {
			if b[i] != values[k] {
				t.Fatalf("buffer %d is invalid", k)
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
	// This benchmarks a typical workflow for a streaming codec. What usually
	// happens is:
	// - a part of the buffer is claimed (here we claim the whole buffer)
	// - we read from the network in the claimed part. Here that is replaced
	//   with a memcpy
	// - the buffer now contains multiple messages that can be decoded
	// - a decode means: commit some bytes, decode them into an internal
	//   type and the consume them
	// - the tail of the buffer might contain a partial message - a `Consume` is
	//   needed in order to make space for the remainder of the message before a
	//   new network `read` is performed.
	//
	// (Arguably, it could be more efficient to Consume only after processing
	// all complete messages in a `ByteBuffer`. However, we currently don't do
	// that in production. As such, this benchmark reflects the improvements
	// that we would get by swapping `ByteBuffer` with `MirroredBuffer` right
	// now).

	var (
		size   = syscall.Getpagesize() * 32
		toCopy = make([]byte, size) // what we copy in the claimed part

		// The number of messages to decode after a Claim+memcpy
		nMessages = []int{1, 2, 4, 8, 16, 32, 64, 128, 256, 512}
	)

	for _, nMessage := range nMessages {
		nMessage := nMessage
		bytesPerMessage := len(toCopy) / nMessage
		b.Run(
			fmt.Sprintf(
				"byte_buffer_%s_%d",
				util.ByteCountSI(int64(size)),
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
				util.ByteCountSI(int64(size)),
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
						b := buf.Claim(size)
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
