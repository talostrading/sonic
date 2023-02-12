package sonic

import (
	"io"
	"os"
	"sync/atomic"
	"syscall"

	"github.com/talostrading/sonic/internal"
	"github.com/talostrading/sonic/sonicerrors"
)

var _ File = &file{}

type file struct {
	ioc    *IO
	fd     int
	pd     internal.PollData
	closed uint32

	// dispatched tracks how callback are currently on the stack.
	// If the fd has a lot of data to read/write and the caller nests
	// read/write calls then we might overflow the stack. In order to not do that
	// we limit the number of dispatched reads to MaxCallbackDispatch.
	// If we hit that limit, we schedule an async read/write which results in clearing the stack.
	dispatched int
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
		if err == syscall.EWOULDBLOCK || err == syscall.EAGAIN {
			return 0, sonicerrors.ErrWouldBlock
		}

		return 0, err
	}

	if n == 0 {
		return 0, io.EOF
	}

	if n < 0 {
		n = 0
	}

	return n, err
}

func (f *file) Write(b []byte) (int, error) {
	n, err := syscall.Write(f.fd, b)

	if err != nil {
		if err == syscall.EWOULDBLOCK || err == syscall.EAGAIN {
			return 0, sonicerrors.ErrWouldBlock
		}

		return 0, err
	}

	if n == 0 {
		return 0, io.EOF
	}

	if n < 0 {
		n = 0
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
	if f.dispatched < MaxCallbackDispatch {
		f.asyncReadNow(b, 0, readAll, func(err error, n int) {
			f.dispatched++
			cb(err, n)
			f.dispatched--
		})
	} else {
		f.scheduleRead(b, 0, readAll, cb)
	}
}

func (f *file) asyncReadNow(b []byte, readBytes int, readAll bool, cb AsyncCallback) {
	n, err := f.Read(b[readBytes:])
	readBytes += n

	// f is a nonblocking fd so if err == ErrWouldBlock
	// then we need to schedule an async read.

	if err == nil && !(readAll && readBytes != len(b)) {
		// If readAll == true then read fully without errors.
		// If readAll == false then read some without errors.
		// We are done.
		cb(nil, readBytes)
		return
	}

	// handles (readAll == false) and (readAll == true && readBytes != len(b)).
	if err == sonicerrors.ErrWouldBlock {
		// If readAll == true then read some without errors.
		// We schedule an asynchronous read.
		f.scheduleRead(b, readBytes, readAll, cb)
	} else {
		cb(err, readBytes)
	}
}

func (f *file) scheduleRead(b []byte, readBytes int, readAll bool, cb AsyncCallback) {
	if f.Closed() {
		cb(io.EOF, 0)
		return
	}

	handler := f.getReadHandler(b, readBytes, readAll, cb)
	f.pd.Set(internal.ReadEvent, handler)

	if err := f.setRead(); err != nil {
		cb(err, readBytes)
	} else {
		f.ioc.pendingReads[&f.pd] = struct{}{}
	}
}

func (f *file) getReadHandler(b []byte, readBytes int, readAll bool, cb AsyncCallback) internal.Handler {
	return func(err error) {
		delete(f.ioc.pendingReads, &f.pd)
		if err != nil {
			cb(err, readBytes)
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
	if f.dispatched < MaxCallbackDispatch {
		f.asyncWriteNow(b, 0, writeAll, func(err error, n int) {
			f.dispatched++
			cb(err, n)
			f.dispatched--
		})
	} else {
		f.scheduleWrite(b, 0, writeAll, cb)
	}
}

func (f *file) asyncWriteNow(b []byte, writtenBytes int, writeAll bool, cb AsyncCallback) {
	n, err := f.Write(b[writtenBytes:])
	writtenBytes += n

	// f is a nonblocking fd so if err == ErrWouldBlock
	// then we need to schedule an async write.

	if err == nil && !(writeAll && writtenBytes != len(b)) {
		// If writeAll == true then wrote fully without errors.
		// If writeAll == false then wrote some without errors.
		// We are done.
		cb(nil, writtenBytes)
		return
	}

	// handles (writeAll == false) and (writeAll == true && writtenBytes != len(b)).
	if err == sonicerrors.ErrWouldBlock {
		// If writeAll == true then wrote some without errors.
		// We schedule an asynchronous write.
		f.scheduleWrite(b, writtenBytes, writeAll, cb)
	} else {
		cb(err, writtenBytes)
	}
}

func (f *file) scheduleWrite(b []byte, writtenBytes int, writeAll bool, cb AsyncCallback) {
	if f.Closed() {
		cb(io.EOF, 0)
		return
	}

	handler := f.getWriteHandler(b, writtenBytes, writeAll, cb)
	f.pd.Set(internal.WriteEvent, handler)

	if err := f.setWrite(); err != nil {
		cb(err, writtenBytes)
	} else {
		f.ioc.pendingWrites[&f.pd] = struct{}{}
	}
}

func (f *file) getWriteHandler(b []byte, writtenBytes int, writeAll bool, cb AsyncCallback) internal.Handler {
	return func(err error) {
		delete(f.ioc.pendingWrites, &f.pd)

		if err != nil {
			cb(err, writtenBytes)
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

	err := f.ioc.poller.Del(f.fd, &f.pd)
	if err != nil {
		return err
	}

	return syscall.Close(f.fd)
}

func (f *file) Closed() bool {
	return atomic.LoadUint32(&f.closed) == 1
}

func (f *file) Seek(offset int64, whence int) (int64, error) {
	return syscall.Seek(f.fd, offset, whence)
}

func (f *file) Cancel() {
	f.cancelReads()
	f.cancelWrites()
}

func (f *file) cancelReads() {
	if f.pd.Flags&internal.ReadFlags == internal.ReadFlags {
		err := f.ioc.poller.DelRead(f.fd, &f.pd)
		if err == nil {
			err = sonicerrors.ErrCancelled
		}
		f.pd.Cbs[internal.ReadEvent](err)
	}
}

func (f *file) cancelWrites() {
	if f.pd.Flags&internal.WriteFlags == internal.WriteFlags {
		err := f.ioc.poller.DelWrite(f.fd, &f.pd)
		if err == nil {
			err = sonicerrors.ErrCancelled
		}
		f.pd.Cbs[internal.WriteEvent](err)
	}
}

func (f *file) RawFd() int {
	return f.fd
}
