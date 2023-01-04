//go:build linux

package internal

import (
	"syscall"
	"testing"
	"unsafe"
)

func BenchmarkRawSyscall(b *testing.B) {
	fd, err := syscall.EpollCreate1(0)
	if err != nil {
		b.Fatal(err)
	}
	events := make([]Event, 128)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, errno := syscall.RawSyscall6(
			syscall.SYS_EPOLL_WAIT,
			uintptr(fd),
			uintptr(unsafe.Pointer(&events[0])),
			uintptr(len(events)),
			uintptr(0), // timeout
			0, 0,
		)
		if errno != 0 {
			var err error = errno
			b.Fatal(err)
		}
	}

	b.ReportAllocs()
}

func BenchmarkSyscall(b *testing.B) {
	fd, err := syscall.EpollCreate1(0)
	if err != nil {
		b.Fatal(err)
	}
	events := make([]Event, 128)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, errno := syscall.Syscall6(
			syscall.SYS_EPOLL_WAIT,
			uintptr(fd),
			uintptr(unsafe.Pointer(&events[0])),
			uintptr(len(events)),
			uintptr(0), // timeout
			0, 0,
		)
		if errno != 0 {
			var err error = errno
			b.Fatal(err)
		}
	}

	b.ReportAllocs()
}
