//go:build linux

package internal

import (
	"os"
	"syscall"
	"unsafe"
)

type EventFd struct {
	fd int
	pd PollData
}

func NewEventFd(nonBlocking bool) (*EventFd, error) {
	var nonBlock uintptr = 0
	if nonBlocking {
		nonBlock = syscall.O_NONBLOCK
	}

	fd, _, err := syscall.Syscall(syscall.SYS_EVENTFD2, 0, nonBlock, 0)
	if err != 0 {
		_ = syscall.Close(int(fd))
		return nil, os.NewSyscallError("eventfd", err)
	}
	e := &EventFd{
		fd: int(fd),
	}
	e.pd.Fd = e.fd
	return e, nil
}

func (e *EventFd) Write(x uint64) (int, error) {
	/* #nosec G103 -- the use of unsafe has been audited */
	return syscall.Write(e.fd, (*(*[8]byte)(unsafe.Pointer(&x)))[:])
}

func (e *EventFd) Read(b []byte) (int, error) {
	return syscall.Read(e.fd, b)
}

func (e *EventFd) Fd() int {
	return e.fd
}

func (e *EventFd) PollData() *PollData {
	return &e.pd
}

func (e *EventFd) Close() error {
	return syscall.Close(e.fd)
}
