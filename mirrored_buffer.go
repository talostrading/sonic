package sonic

import (
	"fmt"
	"syscall"
	"unsafe"
)

type MirroredBuffer struct {
	slice    []byte
	baseAddr unsafe.Pointer
	size     int

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

	// Reserve 2 * size of the process' virtual memory space.
	//
	// This call is needed for its return value, which is a valid mapping
	// address. We use this address with MAP_FIXED to mirror our desired buffer
	// below.
	//
	// If prefault is true, this call also allocates physical memory. This will
	// be visible in the process' resident memory.
	// If prefault is false, no physical memory allocation occurs. Instead, we
	// defer to lazy allocation of pages in RAM through on-demand paging.

	// No file backing, implies -1 as fd.
	flags := syscall.MAP_ANONYMOUS

	// Modifications to the mapping are only visible to the current process, and
	// are not carried through to the underlying file (which is inexistent,
	// hence -1 as fd).
	//
	// Another effect of MAP_PRIVATE is that another mmap call mapping the same
	// virtual address space will not see the changes made to the memory by the
	// first mapping. Under the hood, this is achieved through kernel
	// copy-on-write, which is the default behaviour for MAP_PRIVATE mappings.
	// That means that if the first mapping tries to modify a page shared with
	// another mapping, the page is first copied and then changed. The first
	// mapping will hold the modified copy, the second mapping will refer the
	// initial page. See the MapPrivate example in tests on how this works.
	// TODO write that example.
	flags |= syscall.MAP_PRIVATE

	if prefault {
		// Prefault the mapping, thus forcing physical memory to back it up.
		flags |= syscall.MAP_POPULATE
	}

	b.slice, err = syscall.Mmap(
		-1,                // fd
		0,                 // offset
		2*size,            // size
		syscall.PROT_NONE, // we can't do anything with this mapping
		flags,
	)
	if err != nil {
		return nil, err
	}

	// Now create a shared memory handle. Truncate it to size. This won't
	// allocate but merely set the size of the shared handle. We then map that
	// handle into memory twice: once at offset 0 and once at offset size, both
	// wrt the address of b.slice returned by mmap above.
	name := "/dev/shm/sonic_mirrored_buffer"

	// NOTE: open(/dev/shm) is equivalent to shm_open.
	// TODO test this on a mac. kernel params: kern.sysv.shmmax/kern.sysv.shmall
	// TODO escape hatch if /dev/shm is not present
	fd, err := syscall.Open(
		name,
		syscall.O_CREAT|syscall.O_RDWR,  // file is readable/writeable
		syscall.S_IRUSR|syscall.S_IWUSR, // user can read/write to this file
	)
	if err != nil {
		_ = b.Destroy()
		return nil, fmt.Errorf("could not open %s err=%v", name, err)
	}

	if err := syscall.Truncate(name, int64(size)); err != nil {
		_ = b.Destroy()
		return nil, fmt.Errorf("could not truncate %s err=%v", name, err)
	}

	// Do not persist the file handle after this process exits.
	if err := syscall.Unlink(name); err != nil {
		_ = b.Destroy()
		return nil, fmt.Errorf("could not unlink %s err=%v", name, err)
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
	flags = syscall.MAP_FIXED

	// Share this mapping within the process' scope. This means any modification
	// made to this [0, size] mapping will be visible in the mirror residing at
	// [size, 2 * size]. This would not be possible with MAP_PRIVATE due to the
	// copy-on-write behaviour documented above.
	// TODO give an example in tests.
	flags |= syscall.MAP_SHARED

	// Make the first mapping: offset=0 length=size.
	addr, _, errno := syscall.Syscall6(
		syscall.SYS_MMAP,
		firstAddrPtr,   // addr
		uintptr(size),  // size
		uintptr(prot),  // prot
		uintptr(flags), // flags
		uintptr(fd),    // fd
		0,              // offset
	)
	err = nil
	if errno != 0 {
		err = errno
	}
	if err != nil {
		_ = b.Destroy()
		return nil, err
	}
	if addr != firstAddrPtr {
		_ = b.Destroy()
		return nil, fmt.Errorf("could not mmap first chunk")
	}

	// Make the second mapping of the same file at: offset=size length=size.
	addr, _, errno = syscall.Syscall6(
		syscall.SYS_MMAP,
		secondAddrPtr,
		uintptr(size),  // size
		uintptr(prot),  // prot
		uintptr(flags), // flags
		uintptr(fd),    // fd
		0,              // offset
	)
	err = nil
	if errno != 0 {
		err = errno
	}
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
