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
	slot   internal.Slot
	rw     io.ReadWriter
	rc     syscall.RawConn
	closed uint32

	readReactor  asyncAdapterReadReactor
	writeReactor asyncAdapterWriteReactor
}

type asyncAdapterReadReactor struct {
	adapter   *AsyncAdapter

	b         []byte
	readAll   bool
	cb        AsyncCallback
	readSoFar int
}

func (r *asyncAdapterReadReactor) init(b []byte, readAll bool, cb AsyncCallback) {
	r.b = b
	r.readAll = readAll
	r.cb = cb

	r.readSoFar = 0
}

func (r *asyncAdapterReadReactor) onRead(err error) {
	r.adapter.ioc.Deregister(&r.adapter.slot)
	if err != nil {
		r.cb(err, r.readSoFar)
	} else {
		r.adapter.asyncReadNow(r.b, r.readSoFar, r.readAll, r.cb)
	}
}

type asyncAdapterWriteReactor struct {
	adapter     *AsyncAdapter

	b           []byte
	writeAll    bool
	cb          AsyncCallback
	wroteSoFar int
}

func (r *asyncAdapterWriteReactor) init(b []byte, writeAll bool, cb AsyncCallback) {
	r.b = b
	r.writeAll = writeAll
	r.cb = cb

	r.wroteSoFar = 0
}

func (r *asyncAdapterWriteReactor) onWrite(err error) {
	r.adapter.ioc.Deregister(&r.adapter.slot)
	if err != nil {
		r.cb(err, r.wroteSoFar)
	} else {
		r.adapter.asyncWriteNow(r.b, r.wroteSoFar, r.writeAll, r.cb)
	}
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
		a.slot.Fd = int(fd)
		err := internal.ApplyOpts(int(fd), opts...)

		a.readReactor = asyncAdapterReadReactor{adapter: a}
		a.readReactor.init(nil, false, nil)

		a.writeReactor = asyncAdapterWriteReactor{adapter: a}
		a.writeReactor.init(nil, false, nil)

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
	a.readReactor.init(b, false, cb)
	a.scheduleRead(0, cb)
}

// AsyncReadAll reads data from the underlying file descriptor into b asynchronously.
//
// The provided handler is invoked in the following cases:
//   - an error occurred
//   - the provided buffer has been fully filled after zero or several underlying
//     read(...) operations.
func (a *AsyncAdapter) AsyncReadAll(b []byte, cb AsyncCallback) {
	a.readReactor.init(b, true, cb)
	a.scheduleRead(0, cb)
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

	a.scheduleRead(readBytes, cb)
}

func (a *AsyncAdapter) scheduleRead(readBytes int, cb AsyncCallback) {
	if a.Closed() {
		cb(io.EOF, readBytes)
		return
	}

	a.readReactor.readSoFar = readBytes
	a.slot.Set(internal.ReadEvent, a.readReactor.onRead)

	if err := a.ioc.SetRead(&a.slot); err != nil {
		cb(err, readBytes)
	} else {
		a.ioc.Register(&a.slot)
	}
}

// AsyncWrite writes data from the supplied buffer to the underlying file descriptor asynchronously.
//
// AsyncWrite returns no error on short writes. If you want to ensure that the provided
// buffer is completely written, use AsyncWriteAll.
func (a *AsyncAdapter) AsyncWrite(b []byte, cb AsyncCallback) {
	a.writeReactor.init(b, false, cb)
	a.scheduleWrite(0, cb)
}

// AsyncWriteAll writes data from the supplied buffer to the underlying file descriptor asynchronously.
//
// The provided handler is invoked in the following cases:
//   - an error occurred
//   - the provided buffer has been fully written after zero or several underlying
//     write(...) operations.
func (a *AsyncAdapter) AsyncWriteAll(b []byte, cb AsyncCallback) {
	a.writeReactor.init(b, true, cb)
	a.scheduleWrite(0, cb)
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

	a.scheduleWrite(writtenBytes, cb)
}

func (a *AsyncAdapter) scheduleWrite(writtenBytes int, cb AsyncCallback) {
	if a.Closed() {
		cb(io.EOF, writtenBytes)
		return
	}

	a.writeReactor.wroteSoFar = writtenBytes
	a.slot.Set(internal.WriteEvent, a.writeReactor.onWrite)

	if err := a.ioc.SetWrite(&a.slot); err != nil {
		cb(err, writtenBytes)
	} else {
		a.ioc.Register(&a.slot)
	}
}

func (a *AsyncAdapter) Close() error {
	if !atomic.CompareAndSwapUint32(&a.closed, 0, 1) {
		return io.EOF
	}

	_ = a.ioc.UnsetReadWrite(&a.slot)
	a.ioc.Deregister(&a.slot)

	return syscall.Close(a.slot.Fd)
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
	if a.slot.Events&internal.PollerReadEvent == internal.PollerReadEvent {
		err := a.ioc.poller.DelRead(&a.slot)
		if err == nil {
			err = sonicerrors.ErrCancelled
		}
		a.slot.Handlers[internal.ReadEvent](err)
	}
}

func (a *AsyncAdapter) cancelWrites() {
	if a.slot.Events&internal.PollerWriteEvent == internal.PollerWriteEvent {
		err := a.ioc.poller.DelWrite(&a.slot)
		if err == nil {
			err = sonicerrors.ErrCancelled
		}
		a.slot.Handlers[internal.WriteEvent](err)
	}
}

func (a *AsyncAdapter) RawFd() int {
	return a.slot.Fd
}

func (a *AsyncAdapter) Slot() *internal.Slot {
	return &a.slot
}
