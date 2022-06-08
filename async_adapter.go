package sonic

import (
	"fmt"
	"io"
	"syscall"

	"github.com/talostrading/sonic/internal"
)

type AsyncAdapterHandler func(error, *AsyncAdapter)
type AsyncReaderHandler func(error, *AsyncReader)

var (
	_ FileDescriptor = &AsyncAdapter{}
)

// TODO doc: AsyncAdapter operates on blocking fds, hence we schedule a lot
type AsyncAdapter struct {
	ioc    *IO
	fd     int
	pd     internal.PollData
	rw     io.ReadWriter
	rc     syscall.RawConn
	closed bool
}

func NewAsyncAdapter(ioc *IO, sc syscall.Conn, rw io.ReadWriter, cb AsyncAdapterHandler) {
	rc, err := sc.SyscallConn()
	if err != nil {
		cb(err, nil)
		return
	}

	rc.Control(func(fd uintptr) {
		ifd := int(fd)
		a := &AsyncAdapter{
			fd:  ifd,
			ioc: ioc,
			rw:  rw,
			rc:  rc,
		}
		a.pd.Fd = ifd

		cb(nil, a)
	})
}

func (a *AsyncAdapter) Read(b []byte) (int, error) {
	return a.rw.Read(b)
}

func (a *AsyncAdapter) Write(b []byte) (int, error) {
	return a.rw.Write(b)
}

func (a *AsyncAdapter) AsyncRead(b []byte, cb AsyncCallback) {
	a.scheduleRead(b, 0, false, cb)
}

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

	if err := a.setRead(); err != nil {
		cb(err, readBytes)
	} else {
		a.ioc.pendingReads[&a.pd] = struct{}{}
	}
}

func (a *AsyncAdapter) getReadHandler(b []byte, readBytes int, readAll bool, cb AsyncCallback) internal.Handler {
	return func(err error) {
		delete(a.ioc.pendingReads, &a.pd)

		if err != nil {
			cb(err, readBytes)
		} else {
			a.asyncReadNow(b, readBytes, readAll, cb)
		}
	}
}

func (a *AsyncAdapter) setRead() error {
	return a.ioc.poller.SetRead(a.fd, &a.pd)
}

func (a *AsyncAdapter) AsyncWrite(b []byte, cb AsyncCallback) {
	a.scheduleWrite(b, 0, false, cb)
}

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

	if err := a.setWrite(); err != nil {
		cb(err, writtenBytes)
	} else {
		a.ioc.pendingWrites[&a.pd] = struct{}{}
	}
}

func (a *AsyncAdapter) getWriteHandler(b []byte, writtenBytes int, writeAll bool, cb AsyncCallback) internal.Handler {
	return func(err error) {
		delete(a.ioc.pendingWrites, &a.pd)

		if err != nil {
			cb(err, writtenBytes)
		} else {
			a.asyncWriteNow(b, writtenBytes, writeAll, cb)
		}
	}
}

func (a *AsyncAdapter) setWrite() error {
	return a.ioc.poller.SetWrite(a.fd, &a.pd)
}

func (a *AsyncAdapter) Close() error {
	if a.closed {
		return fmt.Errorf("already closed")
	} else {
		a.closed = true
		return nil
	}
}

func (a *AsyncAdapter) Closed() bool {
	return a.closed
}

func (a *AsyncAdapter) Cancel() {
	a.cancelReads()
	a.cancelWrites()
}

func (a *AsyncAdapter) cancelReads() {
	if a.pd.Flags&internal.ReadFlags == internal.ReadFlags {
		err := a.ioc.poller.DelRead(a.fd, &a.pd)
		if err == nil {
			err = ErrCancelled
		}
		a.pd.Cbs[internal.ReadEvent](err)
	}
}

func (a *AsyncAdapter) cancelWrites() {
	if a.pd.Flags&internal.WriteFlags == internal.WriteFlags {
		err := a.ioc.poller.DelWrite(a.fd, &a.pd)
		if err == nil {
			err = ErrCancelled
		}
		a.pd.Cbs[internal.WriteEvent](err)
	}
}
