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
type baseFd struct {
	ioc   *IO
	rawFd int

	// impl is a concrete implementation of a FileDescriptor which defines the *Read and *Write behaviour.
	impl FileDescriptor
	opts []sonicopts.Option

	pd     internal.PollData
	closed uint32
}

var _ FileDescriptor = &baseFd{}

func newBaseFd(
	ioc *IO,
	rawFd int,
	impl FileDescriptor,
	opts ...sonicopts.Option,
) (fd *baseFd, err error) {
	fd = &baseFd{
		ioc:   ioc,
		rawFd: rawFd,
		impl:  impl,
		opts:  opts,
	}
	fd.pd.Fd = rawFd

	err = internal.ApplyOpts(rawFd, opts...)

	return
}

func (fd *baseFd) Read(b []byte) (int, error) {
	return fd.impl.Read(b)
}

func (fd *baseFd) Write(b []byte) (int, error) {
	return fd.impl.Write(b)
}

func (fd *baseFd) AsyncRead(b []byte, cb AsyncCallback) {
	fd.impl.AsyncRead(b, cb)
}

func (fd *baseFd) AsyncReadAll(b []byte, cb AsyncCallback) {
	fd.impl.AsyncReadAll(b, cb)
}

func (fd *baseFd) AsyncWrite(b []byte, cb AsyncCallback) {
	fd.impl.AsyncWrite(b, cb)
}

func (fd *baseFd) AsyncWriteAll(b []byte, cb AsyncCallback) {
	fd.impl.AsyncWriteAll(b, cb)
}

func (fd *baseFd) CancelReads() {
	if fd.pd.Flags&internal.ReadFlags == internal.ReadFlags {
		err := fd.ioc.poller.DelRead(fd.rawFd, &fd.pd)
		if err == nil {
			err = sonicerrors.ErrCancelled
		}
		fd.pd.Cbs[internal.ReadEvent](err)
	}
}

func (fd *baseFd) CancelWrites() {
	if fd.pd.Flags&internal.WriteFlags == internal.WriteFlags {
		err := fd.ioc.poller.DelWrite(fd.rawFd, &fd.pd)
		if err == nil {
			err = sonicerrors.ErrCancelled
		}
		fd.pd.Cbs[internal.WriteEvent](err)
	}
}

func (fd *baseFd) Close() error {
	if !atomic.CompareAndSwapUint32(&fd.closed, 0, 1) {
		return io.EOF
	}

	err := fd.ioc.poller.Del(fd.rawFd, &fd.pd)
	if err != nil {
		return err
	}

	return syscall.Close(fd.rawFd)
}

func (fd *baseFd) Closed() bool {
	return atomic.LoadUint32(&fd.closed) == 1
}

func (fd *baseFd) asyncReadNow(b []byte, readBytes int, readAll bool, cb AsyncCallback) {
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

func (fd *baseFd) scheduleRead(b []byte, readBytes int, readAll bool, cb AsyncCallback) {
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

func (fd *baseFd) getReadHandler(b []byte, readBytes int, readAll bool, cb AsyncCallback) internal.Handler {
	return func(err error) {
		delete(fd.ioc.pendingReads, &fd.pd)
		if err != nil {
			cb(err, readBytes)
		} else {
			fd.asyncReadNow(b, readBytes, readAll, cb)
		}
	}
}

func (fd *baseFd) setRead() error {
	return fd.ioc.poller.SetRead(fd.rawFd, &fd.pd)
}

func (fd *baseFd) asyncWriteNow(b []byte, writtenBytes int, writeAll bool, cb AsyncCallback) {
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

func (fd *baseFd) scheduleWrite(b []byte, writtenBytes int, writeAll bool, cb AsyncCallback) {
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

func (fd *baseFd) getWriteHandler(b []byte, writtenBytes int, writeAll bool, cb AsyncCallback) internal.Handler {
	return func(err error) {
		delete(fd.ioc.pendingWrites, &fd.pd)

		if err != nil {
			cb(err, 0)
		} else {
			fd.asyncWriteNow(b, writtenBytes, writeAll, cb)
		}
	}
}

func (fd *baseFd) setWrite() error {
	return fd.ioc.poller.SetWrite(fd.rawFd, &fd.pd)
}

func (fd *baseFd) RawFd() int {
	return fd.rawFd
}

var _ FileDescriptor = &nonblockingFd{}

type nonblockingFd struct {
	*baseFd

	// TODO should be a single counter
	// TODO doc
	readDispatch, writeDispatch int
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
	if fd.readDispatch < MaxCallbackDispatch {
		fd.asyncReadNow(b, 0, readAll, func(err error, n int) {
			fd.readDispatch++
			cb(err, n)
			fd.readDispatch--
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
	if fd.writeDispatch < MaxCallbackDispatch {
		fd.asyncWriteNow(b, 0, writeAll, func(err error, n int) {
			fd.writeDispatch++
			cb(err, n)
			fd.writeDispatch--
		})
	} else {
		fd.scheduleWrite(b, 0, writeAll, cb)
	}
}

var _ FileDescriptor = &blockingFd{}

type blockingFd struct {
	*baseFd
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
