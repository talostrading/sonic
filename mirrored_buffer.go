package sonic

import (
	"fmt"
	"syscall"
	"unsafe"
)

type MirroredBuffer struct {
	baseAddr unsafe.Pointer
	size     int
	head     int
	tail     int

	used int
}

func NewMirroredBuffer(size int) (*MirroredBuffer, error) {
	b := &MirroredBuffer{
		baseAddr: unsafe.Pointer(nil),
		size:     size,
		head:     0,
		tail:     0,
	}

	name := "/dev/shm/mirrored_buffer"

	fd, err := syscall.Open(
		name,
		syscall.O_CREAT|syscall.O_RDWR,
		syscall.S_IRUSR|syscall.S_IWUSR,
	)
	if err != nil {
		return nil, fmt.Errorf("could not open %s err=%v", name, err)
	}

	if err := syscall.Truncate(name, int64(size)); err != nil {
		return nil, fmt.Errorf("could not truncate %s err=%v", name, err)
	}

	if err := syscall.Unlink(name); err != nil {
		return nil, fmt.Errorf("could not unlink %s err=%v", name, err)
	}

	slice, err := syscall.Mmap(
		-1,     // fd
		0,      // offset
		2*size, // size
		syscall.PROT_NONE,
		syscall.MAP_ANONYMOUS|syscall.MAP_PRIVATE|syscall.MAP_POPULATE,
	)
	if err != nil {
		return nil, err
	}

	b.baseAddr = unsafe.Pointer(unsafe.SliceData(slice))
	firstAddrPtr := uintptr(b.baseAddr)
	secondAddr := unsafe.Add(b.baseAddr, size)
	secondAddrPtr := uintptr(secondAddr)

	addr, _, errno := syscall.Syscall6(
		syscall.SYS_MMAP,
		firstAddrPtr,
		uintptr(size),
		uintptr(syscall.PROT_READ|syscall.PROT_WRITE),
		uintptr(syscall.MAP_FIXED|syscall.MAP_PRIVATE),
		uintptr(fd),
		0,
	)
	err = nil
	if errno != 0 {
		err = errno
	}
	if err != nil {
		return nil, err
	}
	if addr != firstAddrPtr {
		return nil, fmt.Errorf("could not mmap first chunk")
	}

	addr, _, errno = syscall.Syscall6(
		syscall.SYS_MMAP,
		secondAddrPtr,
		uintptr(size),
		uintptr(syscall.PROT_READ|syscall.PROT_WRITE),
		uintptr(syscall.MAP_FIXED|syscall.MAP_PRIVATE),
		uintptr(fd),
		0,
	)
	err = nil
	if errno != 0 {
		err = errno
	}
	if err != nil {
		return nil, err
	}
	if addr != secondAddrPtr {
		return nil, fmt.Errorf("could not mmap second chunk")
	}

	if err := syscall.Close(fd); err != nil {
		return nil, err
	}

	return b, nil
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
	return unsafe.Slice((*byte)(b.baseAddr), n)
}

func (b *MirroredBuffer) Commit(n int) int {
	if free := b.FreeSpace(); n > free {
		n = free
	}
	b.used += n
	b.tail += n
	if (b.tail >= b.size) {
		b.tail -= b.size
	}
	return n 
}

func (b *MirroredBuffer) Consume(n int) int {
	if used := b.UsedSpace(); n > used {
		n = used
	}
	if (n == 0) {
		return 0
	}
	b.used -= n
	b.head += n
	if (b.head >= b.size) {
		b.head -= b.size
	}
	return n
}
