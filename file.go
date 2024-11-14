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
	slot   internal.Slot
	closed uint32
}

func newFile(ioc *IO, fd int) *file {
	f := &file{
		ioc:  ioc,
		slot: internal.Slot{Fd: fd},
	}
	atomic.StoreUint32(&f.closed, 0)
	return f
}

func Open(ioc *IO, path string, flags int, mode os.FileMode) (File, error) {
	fd, err := syscall.Open(path, flags, uint32(mode))
	if err != nil {
		return nil, err
	}

	return newFile(ioc, fd), nil
}

func (f *file) Read(b []byte) (int, error) {
	n, err := syscall.Read(f.slot.Fd, b)

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
	n, err := syscall.Write(f.slot.Fd, b)

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
	if f.ioc.Dispatched < MaxCallbackDispatch {
		f.asyncReadNow(b, 0, readAll, func(err error, n int) {
			f.ioc.Dispatched++
			cb(err, n)
			f.ioc.Dispatched--
		})
	} else {
		f.scheduleRead(b, 0, readAll, cb)
	}
}

func (f *file) asyncReadNow(b []byte, readSoFar int, readAll bool, cb AsyncCallback) {
	n, err := f.Read(b[readSoFar:])
	readSoFar += n

	// f is a nonblocking fd so if err == ErrWouldBlock
	// then we need to schedule an async read.

	if err == nil && !(readAll && readSoFar != len(b)) {
		// If readAll == true then read fully without errors.
		// If readAll == false then read some without errors.
		// We are done.
		cb(nil, readSoFar)
		return
	}

	// handles (readAll == false) and (readAll == true && readSoFar != len(b)).
	if err == sonicerrors.ErrWouldBlock {
		// If readAll == true then read some without errors.
		// We schedule an asynchronous read.
		f.scheduleRead(b, readSoFar, readAll, cb)
	} else {
		cb(err, readSoFar)
	}
}

func (f *file) scheduleRead(b []byte, readSoFar int, readAll bool, cb AsyncCallback) {
	if f.Closed() {
		cb(io.EOF, 0)
		return
	}

	handler := f.getReadHandler(b, readSoFar, readAll, cb)
	f.slot.Set(internal.ReadEvent, handler)

	if err := f.ioc.SetRead(&f.slot); err != nil {
		cb(err, readSoFar)
	} else {
		f.ioc.Register(&f.slot)
	}
}

func (f *file) getReadHandler(b []byte, readSoFar int, readAll bool, cb AsyncCallback) internal.Handler {
	return func(err error) {
		f.ioc.Deregister(&f.slot)
		if err != nil {
			cb(err, readSoFar)
		} else {
			f.asyncReadNow(b, readSoFar, readAll, cb)
		}
	}
}

func (f *file) AsyncWrite(b []byte, cb AsyncCallback) {
	f.asyncWrite(b, false, cb)
}

func (f *file) AsyncWriteAll(b []byte, cb AsyncCallback) {
	f.asyncWrite(b, true, cb)
}

func (f *file) asyncWrite(b []byte, writeAll bool, cb AsyncCallback) {
	if f.ioc.Dispatched < MaxCallbackDispatch {
		f.asyncWriteNow(b, 0, writeAll, func(err error, n int) {
			f.ioc.Dispatched++
			cb(err, n)
			f.ioc.Dispatched--
		})
	} else {
		f.scheduleWrite(b, 0, writeAll, cb)
	}
}

func (f *file) asyncWriteNow(b []byte, wroteSoFar int, writeAll bool, cb AsyncCallback) {
	n, err := f.Write(b[wroteSoFar:])
	wroteSoFar += n

	if err == nil && !(writeAll && wroteSoFar != len(b)) {
		// If writeAll == true then we wrote fully without errors.
		// If writeAll == false then we wrote some without errors.
		cb(nil, wroteSoFar)
		return
	}

	// Handles (writeAll == false) and (writeAll == true && wroteSoFar != len(b)).
	if err == sonicerrors.ErrWouldBlock {
		f.scheduleWrite(b, wroteSoFar, writeAll, cb)
	} else {
		cb(err, wroteSoFar)
	}
}

func (f *file) scheduleWrite(b []byte, wroteSoFar int, writeAll bool, cb AsyncCallback) {
	if f.Closed() {
		cb(io.EOF, 0)
		return
	}

	handler := f.getWriteHandler(b, wroteSoFar, writeAll, cb)
	f.slot.Set(internal.WriteEvent, handler)

	if err := f.ioc.SetWrite(&f.slot); err != nil {
		cb(err, wroteSoFar)
	} else {
		f.ioc.Register(&f.slot)
	}
}

func (f *file) getWriteHandler(b []byte, wroteSoFar int, writeAll bool, cb AsyncCallback) internal.Handler {
	return func(err error) {
		f.ioc.Deregister(&f.slot)

		if err != nil {
			cb(err, wroteSoFar)
		} else {
			f.asyncWriteNow(b, wroteSoFar, writeAll, cb)
		}
	}
}

func (f *file) Close() error {
	if !atomic.CompareAndSwapUint32(&f.closed, 0, 1) {
		return io.EOF
	}

	if err := f.ioc.UnsetReadWrite(&f.slot); err != nil {
		return err
	}
	f.ioc.Deregister(&f.slot)

	return syscall.Close(f.slot.Fd)
}

func (f *file) Closed() bool {
	return atomic.LoadUint32(&f.closed) == 1
}

func (f *file) Seek(offset int64, whence int) (int64, error) {
	return syscall.Seek(f.slot.Fd, offset, whence)
}

func (f *file) Cancel() {
	f.cancelReads()
	f.cancelWrites()
}

func (f *file) cancelReads() {
	if f.slot.Events&internal.PollerReadEvent == internal.PollerReadEvent {
		err := f.ioc.poller.DelRead(&f.slot)
		if err == nil {
			err = sonicerrors.ErrCancelled
		}
		f.slot.Handlers[internal.ReadEvent](err)
	}
}

func (f *file) cancelWrites() {
	if f.slot.Events&internal.PollerWriteEvent == internal.PollerWriteEvent {
		err := f.ioc.poller.DelWrite(&f.slot)
		if err == nil {
			err = sonicerrors.ErrCancelled
		}
		f.slot.Handlers[internal.WriteEvent](err)
	}
}

func (f *file) RawFd() int {
	return f.slot.Fd
}
