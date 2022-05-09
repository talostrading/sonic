package sonic

import (
	"errors"
	"io"
	"os"
	"sync/atomic"
	"syscall"

	"github.com/talostrading/sonic/internal"
)

var _ File = &file{}

// TODO write sync and nonblocking as much as possible

type file struct {
	ioc    *IO
	fd     int
	pd     internal.PollData
	closed uint32
}

func Open(ioc *IO, path string, flags int, mode os.FileMode) (File, error) {
	fd, err := syscall.Open(path, flags, uint32(mode))
	if err != nil {
		return nil, err
	}

	f := &file{
		ioc: ioc,
		fd:  fd,
	}
	f.pd.Fd = fd
	return f, nil
}

func (f *file) Read(b []byte) (int, error) {
	n, err := syscall.Read(f.fd, b)
	if err != nil {
		if err == syscall.EWOULDBLOCK {
			if n == 0 {
				err = ErrWouldBlock
			} else {
				err = errors.New("blocked on read, but read something, weird") // TODO maybe standardize these errors
			}
			return n, ErrWouldBlock
		} else {
			if n == 0 {
				err = io.EOF
			}
		}
	}
	return n, err
}

func (f *file) Write(b []byte) (int, error) {
	n, err := syscall.Write(f.fd, b)
	if err != nil {
		if err == syscall.EWOULDBLOCK {
			if n == 0 {
				err = ErrWouldBlock
			} else {
				err = errors.New("blocked on write, but wrote something, weird") // TODO maybe standardize these errors
			}
			return n, ErrWouldBlock
		} else {
			if n == 0 {
				err = io.EOF
			}
		}
	}
	return n, err
}

func (f *file) AsyncRead(b []byte, cb AsyncCallback) {
	// TODO
}

func (f *file) AsyncWrite(b []byte, cb AsyncCallback) {
	// TODO
}

func (f *file) Close() error {
	if !atomic.CompareAndSwapUint32(&f.closed, 0, 1) {
		return io.EOF
	}

	err := f.ioc.poller.Del(f.fd, &f.pd) // TODO don't pass the fd as it's already in the PollData instance
	syscall.Close(f.fd)
	return err
}
