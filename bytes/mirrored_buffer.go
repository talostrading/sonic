package bytes

import (
	"fmt"
	"os"
	"path"
	"syscall"
	"unsafe"
)

var mirroredBufferLocations = []string{
	"/dev/shm",
	"/tmp",
}

const mirroredBufferName = "sonic_mirrored_buffer"

type MirroredBuffer struct {
	slice    []byte
	baseAddr unsafe.Pointer
	size     int
	name     string

	// state
	head int
	tail int
	used int
}

// NewMirroredBuffer returns a mirrored buffer of at least the passed size.
//
// The passed size should be a multiple of the system's page size. If it is not,
// the buffer's size will be rounded up to the nearest multiple of the system's
// page size, which is usually equal to 4KiB.
//
// If prefault is true, the memory used by the buffer is physically backed after
// this call. This results in an immediate allocation, visible in the process'
// resident memory. Prefaulting can be done post initialization through
// MirroredBuffer.Prefault().
//
// This function must no be called concurrently.
func NewMirroredBuffer(size int, prefault bool) (b *MirroredBuffer, err error) {
	pageSize := syscall.Getpagesize()
	if remainder := size % pageSize; remainder > 0 {
		size += pageSize - remainder
	}

	b = &MirroredBuffer{
		slice:    nil,
		baseAddr: unsafe.Pointer(nil),
		size:     size,

		head: 0,
		tail: 0,
		used: 0,
	}

	b.slice, err = mmapAllocate(2*size, prefault)
	if err != nil {
		return nil, err
	}

	// TODO location should be logged to syslog
	for _, location := range mirroredBufferLocations {
		if _, err = os.Stat(location); err == nil {
			b.name = path.Join(location, mirroredBufferName)
			break
		}
	}
	if err != nil {
		_ = b.Destroy()
		return nil, fmt.Errorf(
			"could not create mirrored buffer, tried %v, err=%v",
			mirroredBufferLocations,
			err,
		)
	}
	if _, err := os.Stat(b.name); err == nil {
		fmt.Println()
		return nil, fmt.Errorf(
			"cannot create mirrored buffer, %s already exists",
			b.name,
		)
	}

	// Now create a shared memory handle. Truncate it to size. This won't
	// allocate but merely set the size of the shared handle. We then map that
	// handle into memory twice: once at offset 0 and once at offset size, both
	// wrt the address of b.slice returned by mmap above.
	//
	// NOTE: we need a well defined handle, more specifically a file, to mirror.
	// Mirroring an anonymous mapping twice won't work. Each
	// MAP_ANONYMOUS | MAP_SHARED mapping is unique - no pages are shared with
	// any other mapping.
	fd, err := syscall.Open(
		b.name,
		syscall.O_CREAT|syscall.O_RDWR,  // file is readable/writeable
		syscall.S_IRUSR|syscall.S_IWUSR, // user can read/write to this file
	)
	if err != nil {
		_ = b.Destroy()
		return nil, fmt.Errorf("could not open %s err=%v", b.name, err)
	}

	if err := syscall.Truncate(b.name, int64(size)); err != nil {
		_ = b.Destroy()
		return nil, fmt.Errorf("could not truncate %s err=%v", b.name, err)
	}

	// Do not persist the file handle after this process exits.
	if err := syscall.Unlink(b.name); err != nil {
		_ = b.Destroy()
		return nil, fmt.Errorf("could not unlink %s err=%v", b.name, err)
	}

	// We now map the shared memory file twice at fixed addresses wrt the
	// b.slice above.
	b.baseAddr = unsafe.Pointer(unsafe.SliceData(b.slice))
	firstAddrPtr := uintptr(b.baseAddr)
	secondAddr := unsafe.Add(b.baseAddr, size)
	secondAddrPtr := uintptr(secondAddr)

	if int(secondAddrPtr)-int(firstAddrPtr) != size {
		return nil, fmt.Errorf("could not compute offset addresses for chunks")
	}

	// Can read/write to this memory.
	prot := syscall.PROT_READ | syscall.PROT_WRITE

	// Force the mapping to start at baseAddr.
	flags := syscall.MAP_FIXED

	// Share this mapping within the process' scope. This means any modification
	// made to this [0, size] mapping will be visible in the mirror residing at
	// [size, 2 * size]. This would not be possible with MAP_PRIVATE due to the
	// copy-on-write behaviour documented above.
	//
	// See TestMirroredBufferMmapBehaviour for a concrete example.
	flags |= syscall.MAP_SHARED

	// Make the first mapping: offset=0 length=size.
	addr, err := mmapRaw(
		firstAddrPtr,
		uintptr(size),
		uintptr(prot),
		uintptr(flags),
		uintptr(fd),
		0,
	)
	if err != nil {
		_ = b.Destroy()
		return nil, err
	}
	if addr != firstAddrPtr {
		_ = b.Destroy()
		return nil, fmt.Errorf("could not mmap first chunk")
	}

	// Make the second mapping of the same file at: offset=size length=size.
	addr, err = mmapRaw(
		secondAddrPtr,
		uintptr(size),
		uintptr(prot),
		uintptr(flags),
		uintptr(fd),
		0,
	)
	if err != nil {
		_ = b.Destroy()
		return nil, err
	}
	if addr != secondAddrPtr {
		_ = b.Destroy()
		return nil, fmt.Errorf("could not mmap second chunk")
	}

	// We can safely close this file descriptor per the mmap spec. Combined with
	// the unlink syscall above, this makes it possible to reuse the
	// name=/dev/shm/mirrored_buffer accross NewMirroredBuffer calls.
	if err := syscall.Close(fd); err != nil {
		_ = b.Destroy()
		return nil, err
	}

	return b, nil
}

// Prefault the buffer, forcing physical memory allocation.
func (b *MirroredBuffer) Prefault() {
	for i := range b.slice {
		b.slice[i] = 0
	}
}

func (b *MirroredBuffer) FreeSpace() int {
	return b.size - b.used
}

func (b *MirroredBuffer) UsedSpace() int {
	return b.used
}

func (b *MirroredBuffer) Claim(n int) []byte {
	if free := b.FreeSpace(); n > free {
		n = free
	}
	if n == 0 {
		return nil
	}
	claim := b.slice[b.tail:]
	return claim[:n]
}

func (b *MirroredBuffer) Commit(n int) int {
	if free := b.FreeSpace(); n > free {
		n = free
	}
	b.used += n
	b.tail += n
	if b.tail >= b.size {
		b.tail -= b.size
	}
	return n
}

func (b *MirroredBuffer) Consume(n int) int {
	if used := b.UsedSpace(); n > used {
		n = used
	}
	if n == 0 {
		return 0
	}
	b.used -= n
	b.head += n
	if b.head >= b.size {
		b.head -= b.size
	}
	return n
}

func (b *MirroredBuffer) Full() bool {
	return b.used > 0 && b.head == b.tail
}

func (b *MirroredBuffer) Destroy() error {
	return syscall.Munmap(b.slice)
}

func (b *MirroredBuffer) Head() []byte {
	return b.slice[:b.size]
}

func (b *MirroredBuffer) Size() int {
	return b.size
}

func (b *MirroredBuffer) Reset() {
	b.head = 0
	b.tail = 0
	b.used = 0
}

func (b *MirroredBuffer) FilesystemName() string {
	return b.name
}
