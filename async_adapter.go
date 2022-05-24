package sonic

import (
	"fmt"
	"io"
	"syscall"

	"github.com/talostrading/sonic/internal"
)

type AsyncAdapterHandler func(error, *AsyncAdapter)
type AsyncReaderHandler func(error, *AsyncReader)

var _ AsyncReadWriter = &AsyncAdapter{}

type AsyncAdapter struct {
	ioc *IO
	fd  int
	pd  internal.PollData
	rw  io.ReadWriter
	rc  syscall.RawConn
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

func (a *AsyncAdapter) AsyncRead(b []byte, cb AsyncCallback) {
	a.pd.Set(internal.ReadEvent, func(err error) {
		if err != nil {
			cb(err, 0)
		} else {
			n, err := a.rw.Read(b)
			cb(err, n)
		}
	})
	fmt.Println("reading from b", len(b))
	a.ioc.poller.SetRead(a.fd, &a.pd)
}

func (a *AsyncAdapter) AsyncReadAll(b []byte, cb AsyncCallback) {
	panic("implement me")
}

func (a *AsyncAdapter) AsyncWrite(b []byte, cb AsyncCallback) {
	a.pd.Set(internal.WriteEvent, func(err error) {
		if err != nil {
			cb(err, 0)
		} else {
			n, err := a.rw.Write(b)
			cb(err, n)
		}
	})
	a.ioc.poller.SetWrite(a.fd, &a.pd)
}

func (a *AsyncAdapter) AsyncWriteAll(b []byte, cb AsyncCallback) {
	panic("implement me")
}
