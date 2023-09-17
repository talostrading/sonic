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
	head     int
	tail     int

	used int
}

func NewMirroredBuffer(size int, prefault bool) (b *MirroredBuffer, err error) {
	// TODO force size to be a multiple of pages size.

	b = &MirroredBuffer{
		baseAddr: unsafe.Pointer(nil),
		size:     size,
		head:     0,
		tail:     0,
	}

	// Reserve 2 * size of the process' virtual memory space.
	//
	// This call is needed for its return value, which is a valid mapping
	// address. We use this address with MAP_FIXED to mirror our desired buffer
	// below.
	//
	// If prefault is true, this call also allocates. If prefault is false, no
	// allocation occurs. Instead, we defer to lazy allocation of pages in RAM
	// through on-demand paging.

	// No file backing, implies -1 as fd.
	flags := syscall.MAP_ANONYMOUS

	// Modifications to the mapping are only visible to the current process, and
	// are not carried through to the underlying file (which is inexistent,
	// hence -1 as fd)
	flags |= syscall.MAP_PRIVATE

	if prefault {
		// Prefault the mapping, thus forcing physical memory to back it up.
		flags |= syscall.MAP_POPULATE
	}

	b.slice, err = syscall.Mmap(
		-1,                // fd
		0,                 // offset
		2*size,            // size
		syscall.PROT_NONE, // we can't do anything with this mapping.
		flags,
	)
	if err != nil {
		return nil, err
	}

	// Now create a shared memory handle. Truncate it to size. This won't
	// allocate but merely set the size of the shared handle. We then map that
	// handle into memory twice: once at offset 0 and once at offset size, both
	// wrt the address of b.slice returned by mmap above.
	name := "/dev/shm/mirrored_buffer"

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

	// Force the mapping to start at addr.
	flags = syscall.MAP_FIXED

	// Do not allow sharing the mapping across processes. Do not carry the
	// updates to the underlying file - this is already the case as we use
	// /dev/shm which is a temporary file system (tmpfs) backed up by RAM.
	// fsync is no-op there.
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

	// We can safely closed this file descriptor per mmap manual.
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
