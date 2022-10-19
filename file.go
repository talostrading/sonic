package sonic

import (
	"github.com/talostrading/sonic/sonicopts"
	"os"
	"syscall"
)

var (
	_ File = &file{}
)

type file struct {
	FileDescriptor
}

func Open(
	ioc *IO,
	path string,
	flags int,
	mode os.FileMode,
	opts ...sonicopts.Option,
) (File, error) {
	rawFd, err := syscall.Open(path, flags, uint32(mode))
	if err != nil {
		return nil, err
	}

	f := &file{}
	f.FileDescriptor, err = NewFileDescriptor(ioc, rawFd)

	return f, nil
}

func (f *file) Seek(offset int64, whence int) (int64, error) {
	return syscall.Seek(f.RawFd(), offset, whence)
}

// TODO file_test.go
