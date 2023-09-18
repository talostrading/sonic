package bytes

import "syscall"

// A raw mmap syscall wrapper. Should be used to remmap already mmaped areas,
// meaning baseAddr should be a valid address returned by a previous
// syscall.Mmap call.
func mmapRaw(
	baseAddr,
	size,
	prot,
	flags,
	fd,
	offset uintptr,
) (uintptr, error) {
	if baseAddr == 0 {
		// A baseaddr == 0 means the mmap call is allowed to place the mapping
		// at any valid address. This mapping returns memory that must be
		// managed by the Go runtime. Using this raw syscall wrapper won't
		// result in that which will lead to incorrect programs.
		panic("use syscall.Mmap instead")
	}

	// Make the second mapping of the same file at: offset=size length=size.
	addr, _, errno := syscall.Syscall6(
		syscall.SYS_MMAP,
		baseAddr,
		size,
		prot,
		flags,
		fd,
		offset,
	)
	var err error = nil
	if errno != 0 {
		err = errno
	}
	return addr, err
}
