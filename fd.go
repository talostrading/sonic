package sonic

import (
	"github.com/talostrading/sonic/internal"
	"github.com/talostrading/sonic/sonicerrors"
	"github.com/talostrading/sonic/sonicopts"
	"io"
	"sync/atomic"
	"syscall"
)

var (
	_ FileDescriptor = &nonblockingFd{}
)

type nonblockingFd struct {
	ioc    *IO
	rawFd  int
	pd     internal.PollData
	closed uint32

	readDispatch, writeDispatch int
}

func NewFileDescriptor(
	ioc *IO,
	rawFd int,
	opts ...sonicopts.Option,
) (ifd FileDescriptor, err error) {
	nonblocking := false
	for _, opt := range opts {
		switch opt.Type() {
		case sonicopts.TypeNonblocking:
			nonblocking = opt.Value().(bool)
		}
	}

	if nonblocking {
		fd := &nonblockingFd{
			ioc:   ioc,
			rawFd: rawFd,
		}
		fd.pd.Fd = rawFd

		ifd = fd
	} else {
		panic("not supported")
	}

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

func (fd *nonblockingFd) asyncReadNow(b []byte, readBytes int, readAll bool, cb AsyncCallback) {
	n, err := fd.Read(b[readBytes:])
	readBytes += n

	if err == nil && !(readAll && readBytes != len(b)) {
		// fully read
		cb(nil, readBytes)
		return
	}

	if err != nil && err != sonicerrors.ErrWouldBlock {
		cb(err, 0)
		return
	}

	fd.scheduleRead(b, readBytes, readAll, cb)
}

func (fd *nonblockingFd) scheduleRead(b []byte, readBytes int, readAll bool, cb AsyncCallback) {
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

func (fd *nonblockingFd) getReadHandler(b []byte, readBytes int, readAll bool, cb AsyncCallback) internal.Handler {
	return func(err error) {
		delete(fd.ioc.pendingReads, &fd.pd)
		if err != nil {
			cb(err, 0)
		} else {
			fd.asyncReadNow(b, readBytes, readAll, cb)
		}
	}
}

func (fd *nonblockingFd) setRead() error {
	return fd.ioc.poller.SetRead(fd.rawFd, &fd.pd)
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

func (fd *nonblockingFd) asyncWriteNow(b []byte, writtenBytes int, writeAll bool, cb AsyncCallback) {
	n, err := fd.Write(b[writtenBytes:])
	writtenBytes += n

	if err == nil && !(writeAll && writtenBytes != len(b)) {
		// fully written
		cb(nil, writtenBytes)
		return
	}

	if err != nil && err != sonicerrors.ErrWouldBlock {
		cb(err, 0)
		return
	}

	fd.scheduleWrite(b, writtenBytes, writeAll, cb)
}

func (fd *nonblockingFd) scheduleWrite(b []byte, writtenBytes int, writeAll bool, cb AsyncCallback) {
	if fd.Closed() {
		cb(io.EOF, 0)
		return
	}

	handler := fd.getWriteHandler(b, writtenBytes, writeAll, cb)
	fd.pd.Set(internal.WriteEvent, handler)

	if err := fd.setWrite(); err != nil {
		cb(err, 0)
	} else {
		fd.ioc.pendingWrites[&fd.pd] = struct{}{}
	}
}

func (fd *nonblockingFd) getWriteHandler(b []byte, writtenBytes int, writeAll bool, cb AsyncCallback) internal.Handler {
	return func(err error) {
		delete(fd.ioc.pendingWrites, &fd.pd)

		if err != nil {
			cb(err, 0)
		} else {
			fd.asyncWriteNow(b, writtenBytes, writeAll, cb)
		}
	}
}

func (fd *nonblockingFd) setWrite() error {
	return fd.ioc.poller.SetWrite(fd.rawFd, &fd.pd)
}

func (fd *nonblockingFd) Close() error {
	if !atomic.CompareAndSwapUint32(&fd.closed, 0, 1) {
		return io.EOF
	}

	err := fd.ioc.poller.Del(fd.rawFd, &fd.pd)
	if err != nil {
		return err
	}

	return syscall.Close(fd.rawFd)
}

func (fd *nonblockingFd) Closed() bool {
	return atomic.LoadUint32(&fd.closed) == 1
}

func (fd *nonblockingFd) CancelReads() {
	if fd.pd.Flags&internal.ReadFlags == internal.ReadFlags {
		err := fd.ioc.poller.DelRead(fd.rawFd, &fd.pd)
		if err == nil {
			err = sonicerrors.ErrCancelled
		}
		fd.pd.Cbs[internal.ReadEvent](err)
	}
}

func (fd *nonblockingFd) CancelWrites() {
	if fd.pd.Flags&internal.WriteFlags == internal.WriteFlags {
		err := fd.ioc.poller.DelWrite(fd.rawFd, &fd.pd)
		if err == nil {
			err = sonicerrors.ErrCancelled
		}
		fd.pd.Cbs[internal.WriteEvent](err)
	}
}

func (fd *nonblockingFd) RawFd() int {
	return fd.rawFd
}
