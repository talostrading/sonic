//go:build darwin || netbsd || freebsd || openbsd || dragonfly

package bytes

import "syscall"

func mmapAllocate(size int, prefault bool) ([]byte, error) {
	// Reserve `size` of the process' virtual memory space.
	//
	// This call is needed for its return value, which is a valid mapping
	// address. We use this address with MAP_FIXED to mirror our desired buffer
	// below.
	//
	// Prefault is ignored on BSD systems.

	// No file backing, implies -1 as fd.
	flags := syscall.MAP_ANON

	// Modifications to the mapping are only visible to the current process, and
	// are not carried through to the underlying file (which is inexistent,
	// hence -1 as fd).
	//
	// See TestMirroredBufferMmapBehaviour for an explanation of MAP_PRIVATE and
	// MAP_SHARED.
	flags |= syscall.MAP_PRIVATE

	return syscall.Mmap(
		-1,
		0,
		size,
		syscall.PROT_NONE, // we can't do anything with this mapping
		flags,
	)
}
