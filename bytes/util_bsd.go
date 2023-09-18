//go:build darwin || netbsd || freebsd || openbsd || dragonfly

package bytes

import "syscall"

func mmapAllocate(size int, prefault bool /* noop on bsd */) ([]byte, error) {
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
	flags := syscall.MAP_ANON

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

	return syscall.Mmap(
		-1,                // fd
		0,                 // offset
		size,              // size
		syscall.PROT_NONE, // we can't do anything with this mapping
		flags,
	)
}
