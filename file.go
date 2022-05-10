package sonic

import (
	"io"
	"os"
	"sync/atomic"
	"syscall"

	"github.com/talostrading/sonic/internal"
)

var _ File = &file{}

type file struct {
	ioc    *IO
	fd     int
	pd     internal.PollData
	closed uint32

	readDispatch, writeDispatch int
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
			return 0, ErrWouldBlock
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
			return 0, ErrWouldBlock
		} else {
			if n == 0 {
				err = io.EOF
			}
		}
	}
	return n, err
}

func (f *file) AsyncRead(b []byte, cb AsyncCallback) {
	f.asyncRead(b, false, cb)
}

func (f *file) AsyncReadAll(b []byte, cb AsyncCallback) {
	f.asyncRead(b, true, cb)
}

func (f *file) asyncRead(b []byte, readAll bool, cb AsyncCallback) {
	if f.readDispatch < MaxReadDispatch {
		f.asyncReadNow(b, 0, readAll, func(err error, n int) {
			f.readDispatch++
			cb(err, n)
			f.readDispatch--
		})
	} else {
		f.scheduleRead(b, 0, readAll, cb)
	}
}

func (f *file) asyncReadNow(b []byte, readBytes int, readAll bool, cb AsyncCallback) {
	n, err := f.Read(b[readBytes:])
	readBytes += n

	if err == nil && !(readAll && n != len(b)) {
		cb(nil, n)
		return
	}

	if err != nil && err != ErrWouldBlock {
		cb(err, 0)
		return
	}

	f.scheduleRead(b, readBytes, readAll, cb)
}

func (f *file) scheduleRead(b []byte, readBytes int, readAll bool, cb AsyncCallback) {
	if f.Closed() {
		cb(io.EOF, 0)
		return
	}

	handler := f.getReadHandler(b, readBytes, readAll, cb)
	f.pd.Set(internal.ReadEvent, handler)

	if err := f.setRead(); err != nil {
		cb(err, 0)
	} else {
		f.ioc.inflightReads[&f.pd] = struct{}{}
	}
}

func (f *file) getReadHandler(b []byte, readBytes int, readAll bool, cb AsyncCallback) internal.Handler {
	return func(err error) {
		delete(f.ioc.inflightReads, &f.pd)
		if err != nil {
			cb(err, 0)
		} else {
			f.asyncReadNow(b, readBytes, readAll, cb)
		}
	}
}

func (f *file) setRead() error {
	return f.ioc.poller.SetRead(f.fd, &f.pd)
}

func (f *file) AsyncWrite(b []byte, cb AsyncCallback) {
	f.asyncWrite(b, false, cb)
}

func (f *file) AsyncWriteAll(b []byte, cb AsyncCallback) {
	f.asyncWrite(b, true, cb)
}

func (f *file) asyncWrite(b []byte, writeAll bool, cb AsyncCallback) {
	if f.writeDispatch < MaxWriteDispatch {
		f.asyncWriteNow(b, 0, writeAll, func(err error, n int) {
			f.writeDispatch++
			cb(err, n)
			f.writeDispatch--
		})
	} else {
		f.scheduleWrite(b, 0, writeAll, cb)
	}
}

func (f *file) asyncWriteNow(b []byte, writtenBytes int, writeAll bool, cb AsyncCallback) {
	n, err := f.Write(b[writtenBytes:])
	writtenBytes += n

	if err == nil && !(writeAll && n != len(b)) {
		cb(nil, n)
		return
	}

	if err != nil && err != ErrWouldBlock {
		cb(err, 0)
	}

	f.scheduleWrite(b, writtenBytes, writeAll, cb)
}

func (f *file) scheduleWrite(b []byte, writtenBytes int, writeAll bool, cb AsyncCallback) {
	if f.Closed() {
		cb(io.EOF, 0)
		return
	}

	handler := f.getWriteHandler(b, writtenBytes, writeAll, cb)
	f.pd.Set(internal.WriteEvent, handler)

	if err := f.setWrite(); err != nil {
		cb(err, 0)
	} else {
		f.ioc.inflightWrites[&f.pd] = struct{}{}
	}
}

func (f *file) getWriteHandler(b []byte, writtenBytes int, writeAll bool, cb AsyncCallback) internal.Handler {
	return func(err error) {
		delete(f.ioc.inflightWrites, &f.pd)

		if err != nil {
			cb(err, 0)
		} else {
			f.asyncWriteNow(b, writtenBytes, writeAll, cb)
		}
	}
}

func (f *file) setWrite() error {
	return f.ioc.poller.SetWrite(f.fd, &f.pd)
}

func (f *file) Close() error {
	if !atomic.CompareAndSwapUint32(&f.closed, 0, 1) {
		return io.EOF
	}

	err := f.ioc.poller.Del(f.fd, &f.pd) // TODO don't pass the fd as it's already in the PollData instance
	syscall.Close(f.fd)
	return err
}

func (f *file) Closed() bool {
	return atomic.LoadUint32(&f.closed) == 1
}

func (f *file) Seek(offset int64, whence SeekWhence) error {
	_, err := syscall.Seek(f.fd, offset, int(whence))
	return err
}
