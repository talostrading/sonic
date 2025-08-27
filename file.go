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
	ioc          *IO
	slot         internal.Slot
	closed       uint32
	readReactor  fileReadReactor
	writeReactor fileWriteReactor
}

type fileReadReactor struct {
	file *file

	b         []byte
	readAll   bool
	cb        AsyncCallback
	readSoFar int
}

func (r *fileReadReactor) init(b []byte, readAll bool, cb AsyncCallback) {
	r.b = b
	r.readAll = readAll
	r.cb = cb

	r.readSoFar = 0
}

func (r *fileReadReactor) onRead(err error) {
	r.file.ioc.Deregister(&r.file.slot)
	if err != nil {
		r.cb(err, r.readSoFar)
	} else {
		r.file.asyncReadNow(r.b, r.readSoFar, r.readAll, r.cb)
	}
}

type fileWriteReactor struct {
	file *file

	b          []byte
	writeAll   bool
	cb         AsyncCallback
	wroteSoFar int
}

func (r *fileWriteReactor) init(b []byte, writeAll bool, cb AsyncCallback) {
	r.b = b
	r.writeAll = writeAll
	r.cb = cb

	r.wroteSoFar = 0
}

func (r *fileWriteReactor) onWrite(err error) {
	r.file.ioc.Deregister(&r.file.slot)
	if err != nil {
		r.cb(err, r.wroteSoFar)
	} else {
		r.file.asyncWriteNow(r.b, r.wroteSoFar, r.writeAll, r.cb)
	}
}

func newFile(ioc *IO, fd int) *file {
	f := &file{
		ioc:  ioc,
		slot: internal.Slot{Fd: fd},
	}
	atomic.StoreUint32(&f.closed, 0)

	f.readReactor = fileReadReactor{file: f}
	f.readReactor.init(nil, false, nil)

	f.writeReactor = fileWriteReactor{file: f}
	f.writeReactor.init(nil, false, nil)

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
	f.readReactor.init(b, readAll, cb)

	if f.ioc.Dispatched < MaxCallbackDispatch {
		f.asyncReadNow(b, 0, readAll, func(err error, n int) {
			f.ioc.Dispatched++
			cb(err, n)
			f.ioc.Dispatched--
		})
	} else {
		f.scheduleRead(0 /* this is the starting point, we did not read anything yet */, cb)
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
		f.scheduleRead(readSoFar, cb)
	} else {
		cb(err, readSoFar)
	}
}

func (f *file) scheduleRead(readSoFar int, cb AsyncCallback) {
	if f.Closed() {
		cb(io.EOF, 0)
		return
	}

	f.readReactor.readSoFar = readSoFar
	f.slot.Set(internal.ReadEvent, f.readReactor.onRead)

	if err := f.ioc.SetRead(&f.slot); err != nil {
		cb(err, readSoFar)
	} else {
		f.ioc.Register(&f.slot)
	}
}

func (f *file) AsyncWrite(b []byte, cb AsyncCallback) {
	f.asyncWrite(b, false, cb)
}

func (f *file) AsyncWriteAll(b []byte, cb AsyncCallback) {
	f.asyncWrite(b, true, cb)
}

func (f *file) asyncWrite(b []byte, writeAll bool, cb AsyncCallback) {
	f.writeReactor.init(b, writeAll, cb)

	if f.ioc.Dispatched < MaxCallbackDispatch {
		f.asyncWriteNow(b, 0, writeAll, func(err error, n int) {
			f.ioc.Dispatched++
			cb(err, n)
			f.ioc.Dispatched--
		})
	} else {
		f.scheduleWrite(0 /* this is the starting point, we did not write anything yet */, cb)
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
		f.scheduleWrite(wroteSoFar, cb)
	} else {
		cb(err, wroteSoFar)
	}
}

func (f *file) scheduleWrite(wroteSoFar int, cb AsyncCallback) {
	f.scheduleWriteFunc(wroteSoFar, cb, f.writeReactor.onWrite)
}

func (f *file) scheduleWriteFunc(wroteSoFar int, cb AsyncCallback, handlerFunc func(error)) {
	if f.Closed() {
		cb(io.EOF, 0)
		return
	}

	f.writeReactor.wroteSoFar = wroteSoFar
	f.slot.Set(internal.WriteEvent, handlerFunc)

	if err := f.ioc.SetWrite(&f.slot); err != nil {
		cb(err, wroteSoFar)
	} else {
		f.ioc.Register(&f.slot)
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
