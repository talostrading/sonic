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
	firstAddr := uintptr(b.baseAddr)
	secondAddr := uintptr(b.baseAddr) + uintptr(size)

	addr, _, errno := syscall.Syscall6(
		syscall.SYS_MMAP,
		firstAddr,
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
	if addr != firstAddr {
		return nil, fmt.Errorf("could not mmap first chunk")
	}

	addr, _, errno = syscall.Syscall6(
		syscall.SYS_MMAP,
		secondAddr,
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
	if addr != secondAddr {
		return nil, fmt.Errorf("could not mmap second chunk")
	}

	if err := syscall.Close(fd); err != nil {
		return nil, err
	}

	return b, nil
}

func (b *MirroredBuffer) Claim(n int) ([]byte, int) {
	return nil, 0
}

func (b *MirroredBuffer) Commit() int {
	return 0
}

func (b *MirroredBuffer) Consume() int {
	return 0
}
