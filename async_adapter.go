package sonic

import (
	"io"
	"sync/atomic"
	"syscall"

	"github.com/talostrading/sonic/internal"
	"github.com/talostrading/sonic/sonicerrors"
	"github.com/talostrading/sonic/sonicopts"
)

var (
	_ FileDescriptor = &AsyncAdapter{}
)

type AsyncAdapterHandler func(error, *AsyncAdapter)

// AsyncAdapter is a wrapper around syscall.Conn which enables
// clients to schedule async read and write operations on the
// underlying file descriptor.
type AsyncAdapter struct {
	ioc    *IO
	pd     internal.PollData
	rw     io.ReadWriter
	rc     syscall.RawConn
	closed uint32
}

// NewAsyncAdapter takes in an IO instance and an interface of syscall.Conn and io.ReadWriter
// pertaining to the same object and invokes a completion handler which:
//   - provides the async adapter on successful completion
//   - provides an error if any occurred when async-adapting the provided object
//
// See async_adapter_test.go for examples on how to setup an AsyncAdapter.
func NewAsyncAdapter(
	ioc *IO,
	sc syscall.Conn,
	rw io.ReadWriter,
	cb AsyncAdapterHandler,
	opts ...sonicopts.Option,
) {
	rc, err := sc.SyscallConn()
	if err != nil {
		cb(err, nil)
		return
	}

	err = rc.Control(func(fd uintptr) {
		a := &AsyncAdapter{
			ioc: ioc,
			rw:  rw,
			rc:  rc,
		}
		a.pd.Fd = int(fd)
		err := internal.ApplyOpts(int(fd), opts...)
		cb(err, a)
	})
	if err != nil {
		cb(err, nil)
	}
}

// Read reads data from the underlying file descriptor into b.
func (a *AsyncAdapter) Read(b []byte) (int, error) {
	return a.rw.Read(b)
}

// Write writes data from the supplied buffer to the underlying file descriptor.
func (a *AsyncAdapter) Write(b []byte) (int, error) {
	return a.rw.Write(b)
}

// AsyncRead reads data from the underlying file descriptor into b asynchronously.
//
// AsyncRead returns no error on short reads. If you want to ensure that the provided
// buffer is completely filled, use AsyncReadAll.
func (a *AsyncAdapter) AsyncRead(b []byte, cb AsyncCallback) {
	a.scheduleRead(b, 0, false, cb)
}

// AsyncReadAll reads data from the underlying file descriptor into b asynchronously.
//
// The provided handler is invoked in the following cases:
//   - an error occurred
//   - the provided buffer has been fully filled after zero or several underlying
//     read(...) operations.
func (a *AsyncAdapter) AsyncReadAll(b []byte, cb AsyncCallback) {
	a.scheduleRead(b, 0, true, cb)
}

func (a *AsyncAdapter) asyncReadNow(b []byte, readBytes int, readAll bool, cb AsyncCallback) {
	n, err := a.rw.Read(b[readBytes:])
	readBytes += n

	if err == nil && !(readAll && readBytes != len(b)) {
		cb(nil, readBytes)
		return
	}

	if err != nil {
		cb(err, readBytes)
		return
	}

	a.scheduleRead(b, readBytes, readAll, cb)
}

func (a *AsyncAdapter) scheduleRead(b []byte, readBytes int, readAll bool, cb AsyncCallback) {
	if a.Closed() {
		cb(io.EOF, readBytes)
		return
	}

	handler := a.getReadHandler(b, readBytes, readAll, cb)
	a.pd.Set(internal.ReadEvent, handler)

	if err := a.ioc.SetRead(&a.pd); err != nil {
		cb(err, readBytes)
	} else {
		a.ioc.Register(&a.pd)
	}
}

func (a *AsyncAdapter) getReadHandler(b []byte, readBytes int, readAll bool, cb AsyncCallback) internal.Handler {
	return func(err error) {
		a.ioc.Deregister(&a.pd)

		if err != nil {
			cb(err, readBytes)
		} else {
			a.asyncReadNow(b, readBytes, readAll, cb)
		}
	}
}

// AsyncWrite writes data from the supplied buffer to the underlying file descriptor asynchronously.
//
// AsyncWrite returns no error on short writes. If you want to ensure that the provided
// buffer is completely written, use AsyncWriteAll.
func (a *AsyncAdapter) AsyncWrite(b []byte, cb AsyncCallback) {
	a.scheduleWrite(b, 0, false, cb)
}

// AsyncWriteAll writes data from the supplied buffer to the underlying file descriptor asynchronously.
//
// The provided handler is invoked in the following cases:
//   - an error occurred
//   - the provided buffer has been fully written after zero or several underlying
//     write(...) operations.
func (a *AsyncAdapter) AsyncWriteAll(b []byte, cb AsyncCallback) {
	a.scheduleWrite(b, 0, true, cb)
}

func (a *AsyncAdapter) asyncWriteNow(b []byte, writtenBytes int, writeAll bool, cb AsyncCallback) {
	n, err := a.rw.Write(b[writtenBytes:])
	writtenBytes += n

	if err == nil && !(writeAll && writtenBytes != len(b)) {
		cb(nil, writtenBytes)
		return
	}

	if err != nil {
		cb(err, writtenBytes)
		return
	}

	a.scheduleWrite(b, writtenBytes, writeAll, cb)
}

func (a *AsyncAdapter) scheduleWrite(b []byte, writtenBytes int, writeAll bool, cb AsyncCallback) {
	if a.Closed() {
		cb(io.EOF, writtenBytes)
		return
	}

	handler := a.getWriteHandler(b, writtenBytes, writeAll, cb)
	a.pd.Set(internal.WriteEvent, handler)

	if err := a.ioc.SetWrite(&a.pd); err != nil {
		cb(err, writtenBytes)
	} else {
		a.ioc.Register(&a.pd)
	}
}

func (a *AsyncAdapter) getWriteHandler(b []byte, writtenBytes int, writeAll bool, cb AsyncCallback) internal.Handler {
	return func(err error) {
		a.ioc.Deregister(&a.pd)

		if err != nil {
			cb(err, writtenBytes)
		} else {
			a.asyncWriteNow(b, writtenBytes, writeAll, cb)
		}
	}
}

func (a *AsyncAdapter) Close() error {
	if !atomic.CompareAndSwapUint32(&a.closed, 0, 1) {
		return io.EOF
	}

	_ = a.ioc.poller.Del(&a.pd)

	return syscall.Close(a.pd.Fd)
}

func (a *AsyncAdapter) AsyncClose(cb func(err error)) {
	err := a.Close()
	cb(err)
}

func (a *AsyncAdapter) Closed() bool {
	return atomic.LoadUint32(&a.closed) == 1
}

// Cancel cancels any asynchronous operations scheduled on the underlying file descriptor.
func (a *AsyncAdapter) Cancel() {
	a.cancelReads()
	a.cancelWrites()
}

func (a *AsyncAdapter) cancelReads() {
	if a.pd.Events&internal.PollerReadEvent == internal.PollerReadEvent {
		err := a.ioc.poller.DelRead(&a.pd)
		if err == nil {
			err = sonicerrors.ErrCancelled
		}
		a.pd.Handlers[internal.ReadEvent](err)
	}
}

func (a *AsyncAdapter) cancelWrites() {
	if a.pd.Events&internal.PollerWriteEvent == internal.PollerWriteEvent {
		err := a.ioc.poller.DelWrite(&a.pd)
		if err == nil {
			err = sonicerrors.ErrCancelled
		}
		a.pd.Handlers[internal.WriteEvent](err)
	}
}

func (a *AsyncAdapter) RawFd() int {
	return a.pd.Fd
}
