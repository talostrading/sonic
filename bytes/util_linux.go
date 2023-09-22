//go:build linux

package bytes

import "syscall"

func mmapAllocate(size int, prefault bool) ([]byte, error) {
	// Reserve `size` of the process' virtual memory space.
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
	// See TestMirroredBufferMmapBehaviour for an explanation of MAP_PRIVATE and
	// MAP_SHARED.
	flags |= syscall.MAP_PRIVATE

	if prefault {
		// Prefault the mapping, thus forcing physical memory to back it up.
		flags |= syscall.MAP_POPULATE
	}

	return syscall.Mmap(
		-1,
		0,
		size,
		syscall.PROT_NONE, // we can't do anything with this mapping
		flags,
	)
}
