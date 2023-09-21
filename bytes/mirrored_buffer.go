package bytes

// TODO check boost::inteprocess on escape hatches if /dev/shm is not here
// TODO check if there's a way to detect other tmpfs on the system
// TODO check if there is a way to not sync up the mmapped memory with the
// backing file on non-tmpfs/standard filesystems
// TODO ensure there's no major page faults for tmpfs filesystems
// TODO measure the impact of major page faults for non-tmpfs mirrored buffers
// TODO maybe escape hatch to ramfs

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

// MirroredBuffer is a circular FIFO buffer that always returns continuous byte
// slices. It is well suited for writing protocols on top of streaming
// transports such as TCP. This buffer does no memory copies or allocations
// outside of initialization time.
//
// For protocols on top of packet based transports such as UDP, use a BipBuffer
// instead.
//
// A MirroredBuffer maps a shared memory file twice in the process' virtual
// memory space. The mappings are sequential. For example, a MirroredBuffer of
// size `n` will create a shared memory file of size `n` and mmap it twice: once
// at `addr` and once at `addr+n`. As a result, a MirroredBuffer will uses
// `2*n` virtual memory and `n` physical memory.
//
// This double mapping allows us to always get a continuous slice of bytes from
// the buffer. The CPU's memory management unit will do the wrapping for us.
//
// There's a trick employed to make the double mmapping possible - this trick
// gives us the `addr` above. Given that both mappings need to be sequential, we
// need to mmap them at fixed virtual memory addresses. We can't arbitrarily
// choose a virtual address - we have no guarantee that it can hold a mmaping of
// size `2*n`. That's why we let mmap choose it for us by initially mmaping an
// area of size `2*n` with MAP_ANONYMOUS | MAP_PRIVATE and no fd. The returned
// address is then used to mmap the shared memory file twice, consecutively.
// This is done in the locally defined `remap()` function in the constructor.
type MirroredBuffer struct {
	slice    []byte
	size     int
	sizeMask int
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
// resident memory (RSS). Prefaulting can be done post initialization through
// MirroredBuffer.Prefault().
//
// This function must no be called concurrently.
func NewMirroredBuffer(size int, prefault bool) (b *MirroredBuffer, err error) {
	fd := -1
	defer func() {
		// NOTE: We must ensure the mapping is destroyed in case the constructor
		// fails. This means you should never write `err :=` below. Always write
		// `err = `. You can safely return a new error (like with `fmt.Errorf`)
		// - it will get assigned to the error value defined above.
		if err != nil {
			_ = b.Destroy()
		}
		if fd >= 0 {
			// We can safely close this file descriptor per the mmap spec.
			// Combined with the unlink syscall below, this makes it possible to
			// reuse the name=/dev/shm/mirrored_buffer accross NewMirroredBuffer
			// calls.
			_ = syscall.Close(fd)
		}
	}()

	pageSize := syscall.Getpagesize()
	if remainder := size % pageSize; remainder > 0 {
		size += pageSize - remainder
	}
	if size <= 0 {
		return nil, fmt.Errorf("invalid buffer size %d", size)
	}

	b = &MirroredBuffer{
		slice:    nil,
		size:     size,
		sizeMask: size - 1,

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
		return nil, fmt.Errorf(
			"could not create mirrored buffer, tried %v, err=%v",
			mirroredBufferLocations,
			err,
		)
	}
	if _, err = os.Stat(b.name); err == nil {
		return nil, fmt.Errorf(
			"cannot create mirrored buffer, %s already exists",
			b.name,
		)
	}

	fd, err = syscall.Open(
		b.name,
		syscall.O_CREAT|syscall.O_RDWR,  // file is readable/writeable
		syscall.S_IRUSR|syscall.S_IWUSR, // user can read/write to this file
	)
	if err != nil {
		return nil, fmt.Errorf("could not open %s err=%v", b.name, err)
	}

	if err = syscall.Truncate(b.name, int64(size)); err != nil {
		return nil, fmt.Errorf("could not truncate %s err=%v", b.name, err)
	}

	if err = syscall.Unlink(b.name); err != nil {
		return nil, fmt.Errorf("could not unlink %s err=%v", b.name, err)
	}

	// We now map the shared memory file twice at fixed addresses wrt the
	// b.slice above.
	/* #nosec G103 -- the use of unsafe has been audited */
	var (
		firstAddr    = unsafe.Pointer(unsafe.SliceData(b.slice))
		firstAddrPtr = uintptr(firstAddr)

		secondAddr    = unsafe.Add(firstAddr, size)
		secondAddrPtr = uintptr(secondAddr)
	)

	if int(secondAddrPtr)-int(firstAddrPtr) != size {
		return nil, fmt.Errorf(
			"could not compute offset addresses for left and right mappings",
		)
	}

	prot := syscall.PROT_READ | syscall.PROT_WRITE

	// See TestMirroredBufferMmapBehaviour for the behaviour of MAP_SHARED vs
	// MAP_PRIVATE. MAP_FIXED ensures the remap takes place at the address
	// returned by the anoymous mapping above.
	flags := syscall.MAP_FIXED | syscall.MAP_SHARED

	remap := func(
		baseAddr,
		size,
		prot,
		flags,
		fd uintptr,
	) error {
		if baseAddr == 0 {
			return fmt.Errorf(
				"remap: baseAddr must be a valid non-zero virtual address",
			)
		}

		addr, _, errno := syscall.Syscall6(
			syscall.SYS_MMAP,
			baseAddr,
			size,
			prot,
			flags,
			fd,
			0,
		)
		var err error = nil
		if errno != 0 {
			err = errno
		}
		if err == nil && addr != baseAddr {
			return fmt.Errorf(
				"could not remap at address=%d size=%d fd=%d",
				baseAddr, size, fd,
			)
		}
		return err
	}

	// First mapping at offset=0 of length=size.
	if err = remap(
		firstAddrPtr,
		uintptr(size),
		uintptr(prot),
		uintptr(flags),
		uintptr(fd),
	); err != nil {
		return nil, err
	}

	// Second mapping of the same file at offset=size of length=size.
	if err = remap(
		secondAddrPtr,
		uintptr(size),
		uintptr(prot),
		uintptr(flags),
		uintptr(fd),
	); err != nil {
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
	b.tail = (b.tail + n) & b.sizeMask
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
	b.head = (b.head + n) & b.sizeMask
	return n
}

func (b *MirroredBuffer) Full() bool {
	return b.used == b.size
}

func (b *MirroredBuffer) Destroy() (err error) {
	if b.slice != nil {
		err = syscall.Munmap(b.slice)
		if err == nil {
			b.slice = nil
		}
	}
	return nil
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
