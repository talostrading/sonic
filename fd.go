package sonic

import (
	"errors"
	"github.com/talostrading/sonic/internal"
	"github.com/talostrading/sonic/sonicerrors"
	"github.com/talostrading/sonic/sonicopts"
	"io"
	"sync/atomic"
	"syscall"
)

func NewFileDescriptor(
	ioc *IO,
	rawFd int,
	opts ...sonicopts.Option,
) (fd FileDescriptor, err error) {
	nonblocking := false
	for _, opt := range opts {
		switch opt.Type() {
		case sonicopts.TypeNonblocking:
			nonblocking = opt.Value().(bool)
		}
	}

	if nonblocking {
		fd, err = newNonblockingFd(ioc, rawFd, opts...)
	} else {
		fd, err = newBlockingFd(ioc, rawFd, opts...)
	}

	return
}

// baseFd implements behaviour common to all FileDescriptors.
type baseFd[Impl FileDescriptor] struct {
	ioc   *IO
	rawFd int

	// impl is a concrete implementation of a FileDescriptor which defines the *Read and *Write behaviour.
	impl Impl

	opts []sonicopts.Option

	pd     internal.PollData
	closed uint32
}

var _ FileDescriptor = &baseFd[FileDescriptor]{}

func newBaseFd[Impl FileDescriptor](
	ioc *IO,
	rawFd int,
	impl Impl,
	opts ...sonicopts.Option,
) (fd *baseFd[Impl], err error) {
	fd = &baseFd[Impl]{
		ioc:   ioc,
		rawFd: rawFd,
		impl:  impl,
		opts:  opts,
	}
	fd.pd.Fd = rawFd

	err = internal.ApplyOpts(rawFd, opts...)

	return
}

func (fd *baseFd[Impl]) Read(b []byte) (int, error) {
	return fd.impl.Read(b)
}

func (fd *baseFd[Impl]) Write(b []byte) (int, error) {
	return fd.impl.Write(b)
}

func (fd *baseFd[Impl]) AsyncRead(b []byte, cb AsyncCallback) {
	fd.impl.AsyncRead(b, cb)
}

func (fd *baseFd[Impl]) AsyncReadAll(b []byte, cb AsyncCallback) {
	fd.impl.AsyncReadAll(b, cb)
}

func (fd *baseFd[Impl]) AsyncWrite(b []byte, cb AsyncCallback) {
	fd.impl.AsyncWrite(b, cb)
}

func (fd *baseFd[Impl]) AsyncWriteAll(b []byte, cb AsyncCallback) {
	fd.impl.AsyncWriteAll(b, cb)
}

func (fd *baseFd[Impl]) CancelReads() {
	if fd.pd.Flags&internal.ReadFlags == internal.ReadFlags {
		err := fd.ioc.poller.DelRead(fd.rawFd, &fd.pd)
		if err == nil {
			err = sonicerrors.ErrCancelled
		}
		fd.pd.Cbs[internal.ReadEvent](err)
	}
}

func (fd *baseFd[Impl]) CancelWrites() {
	if fd.pd.Flags&internal.WriteFlags == internal.WriteFlags {
		err := fd.ioc.poller.DelWrite(fd.rawFd, &fd.pd)
		if err == nil {
			err = sonicerrors.ErrCancelled
		}
		fd.pd.Cbs[internal.WriteEvent](err)
	}
}

func (fd *baseFd[Impl]) Close() error {
	if !atomic.CompareAndSwapUint32(&fd.closed, 0, 1) {
		return io.EOF
	}

	err := fd.ioc.poller.Del(fd.rawFd, &fd.pd)
	if err != nil {
		return err
	}

	return syscall.Close(fd.rawFd)
}

func (fd *baseFd[Impl]) Closed() bool {
	return atomic.LoadUint32(&fd.closed) == 1
}

func (fd *baseFd[Impl]) Opts() []sonicopts.Option {
	return fd.opts
}

func (fd *baseFd[Impl]) asyncReadNow(b []byte, readBytes int, readAll bool, cb AsyncCallback) {
	n, err := fd.Read(b[readBytes:])
	readBytes += n

	if err == nil && !(readAll && readBytes != len(b)) {
		// If readAll == true then read fully without errors.
		// If readAll == false then read some without errors.
		// We are done.
		cb(nil, readBytes)
		return
	}

	if err == nil || err == sonicerrors.ErrWouldBlock {
		// If readAll == true then read some without errors.
		// If readAll == false then read nothing without errors.
		// We schedule an asynchronous read.
		fd.scheduleRead(b, readBytes, readAll, cb)
	} else {
		cb(err, readBytes)
	}
}

func (fd *baseFd[Impl]) scheduleRead(b []byte, readBytes int, readAll bool, cb AsyncCallback) {
	if fd.Closed() {
		cb(io.EOF, 0)
		return
	}

	handler := fd.getReadHandler(b, readBytes, readAll, cb)
	fd.pd.Set(internal.ReadEvent, handler)

	if err := fd.setRead(); err != nil {
		cb(err, 0)
	} else {
		fd.ioc.pendingReads[&fd.pd] = struct{}{}
	}
}

func (fd *baseFd[Impl]) getReadHandler(b []byte, readBytes int, readAll bool, cb AsyncCallback) internal.Handler {
	return func(err error) {
		delete(fd.ioc.pendingReads, &fd.pd)
		if err != nil {
			cb(err, readBytes)
		} else {
			fd.asyncReadNow(b, readBytes, readAll, cb)
		}
	}
}

func (fd *baseFd[Impl]) setRead() error {
	return fd.ioc.poller.SetRead(fd.rawFd, &fd.pd)
}

func (fd *baseFd[Impl]) asyncWriteNow(b []byte, writtenBytes int, writeAll bool, cb AsyncCallback) {
	n, err := fd.Write(b[writtenBytes:])
	writtenBytes += n

	if err == nil && !(writeAll && writtenBytes != len(b)) {
		// If writeAll == true then wrote fully without errors.
		// If writeAll == false then wrote some without errors.
		// We are done.
		cb(nil, writtenBytes)
		return
	}

	if errors.Is(err, sonicerrors.ErrWouldBlock) {
		// If writeAll == true then wrote some without errors.
		// If writeAll == false then wrote nothing without errors.
		// We schedule an asynchronous write.
		fd.scheduleWrite(b, writtenBytes, writeAll, cb)
	} else {
		cb(err, writtenBytes)
	}
}

func (fd *baseFd[Impl]) scheduleWrite(b []byte, writtenBytes int, writeAll bool, cb AsyncCallback) {
	if fd.Closed() {
		cb(io.EOF, 0)
		return
	}

	handler := fd.getWriteHandler(b, writtenBytes, writeAll, cb)
	fd.pd.Set(internal.WriteEvent, handler)

	if err := fd.setWrite(); err != nil {
		cb(err, writtenBytes)
	} else {
		fd.ioc.pendingWrites[&fd.pd] = struct{}{}
	}
}

func (fd *baseFd[Impl]) getWriteHandler(b []byte, writtenBytes int, writeAll bool, cb AsyncCallback) internal.Handler {
	return func(err error) {
		delete(fd.ioc.pendingWrites, &fd.pd)

		if err != nil {
			cb(err, 0)
		} else {
			fd.asyncWriteNow(b, writtenBytes, writeAll, cb)
		}
	}
}

func (fd *baseFd[Impl]) setWrite() error {
	return fd.ioc.poller.SetWrite(fd.rawFd, &fd.pd)
}

func (fd *baseFd[Impl]) RawFd() int {
	return fd.rawFd
}

var _ FileDescriptor = &nonblockingFd{}

type nonblockingFd struct {
	*baseFd[*nonblockingFd]

	// dispatched keeps track of how many callbacks have been placed onto the stack consecutively, and hence invoked
	// immediately. If dispatched >= MaxCallbackDispatch, callbacks will be scheduled to run asynchronously instead
	// being invoked immediately.
	//
	// This is required because, by default, Go has a 1GB limit for its stack size. If the number of callbacks invoked
	// consecutively is not bound, a stack overflow will occur.
	dispatched int
}

func newNonblockingFd(ioc *IO, rawFd int, opts ...sonicopts.Option) (fd *nonblockingFd, err error) {
	fd = &nonblockingFd{}
	fd.baseFd, err = newBaseFd(ioc, rawFd, fd, opts...)
	return
}

func (fd *nonblockingFd) Read(b []byte) (int, error) {
	n, err := syscall.Read(fd.rawFd, b)

	if err != nil {
		if err == syscall.EWOULDBLOCK || err == syscall.EAGAIN {
			return 0, sonicerrors.ErrWouldBlock
		}

		// TODO maybe not 0 here cause that means the half-read part of the conn was closed? (check how it is on a raw syscall)
		// above applies to writes as well
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

func (fd *nonblockingFd) Write(b []byte) (int, error) {
	n, err := syscall.Write(fd.rawFd, b)

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

func (fd *nonblockingFd) AsyncRead(b []byte, cb AsyncCallback) {
	fd.asyncRead(b, false, cb)
}

func (fd *nonblockingFd) AsyncReadAll(b []byte, cb AsyncCallback) {
	fd.asyncRead(b, true, cb)
}

func (fd *nonblockingFd) asyncRead(b []byte, readAll bool, cb AsyncCallback) {
	if fd.dispatched < MaxCallbackDispatch {
		fd.asyncReadNow(b, 0, readAll, func(err error, n int) {
			fd.dispatched++
			cb(err, n)
			fd.dispatched--
		})
	} else {
		fd.scheduleRead(b, 0, readAll, cb)
	}
}

func (fd *nonblockingFd) AsyncWrite(b []byte, cb AsyncCallback) {
	fd.asyncWrite(b, false, cb)
}

func (fd *nonblockingFd) AsyncWriteAll(b []byte, cb AsyncCallback) {
	fd.asyncWrite(b, true, cb)
}

func (fd *nonblockingFd) asyncWrite(b []byte, writeAll bool, cb AsyncCallback) {
	if fd.dispatched < MaxCallbackDispatch {
		fd.asyncWriteNow(b, 0, writeAll, func(err error, n int) {
			fd.dispatched++
			cb(err, n)
			fd.dispatched--
		})
	} else {
		fd.scheduleWrite(b, 0, writeAll, cb)
	}
}

var _ FileDescriptor = &blockingFd{}

type blockingFd struct {
	*baseFd[*blockingFd]
}

func newBlockingFd(ioc *IO, rawFd int, opts ...sonicopts.Option) (fd *blockingFd, err error) {
	fd = &blockingFd{}
	fd.baseFd, err = newBaseFd(ioc, rawFd, fd, opts...)
	return
}

func (fd *blockingFd) Read(b []byte) (int, error) {
	n, err := syscall.Read(fd.rawFd, b)

	if err != nil {
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

func (fd *blockingFd) Write(b []byte) (int, error) {
	n, err := syscall.Write(fd.rawFd, b)

	if err != nil {
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

func (fd *blockingFd) AsyncRead(b []byte, cb AsyncCallback) {
	fd.scheduleRead(b, 0, false, cb)
}

func (fd *blockingFd) AsyncReadAll(b []byte, cb AsyncCallback) {
	fd.scheduleRead(b, 0, true, cb)
}

func (fd *blockingFd) AsyncWrite(b []byte, cb AsyncCallback) {
	fd.scheduleWrite(b, 0, false, cb)
}

func (fd *blockingFd) AsyncWriteAll(b []byte, cb AsyncCallback) {
	fd.scheduleWrite(b, 0, true, cb)
}
