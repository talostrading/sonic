package sonic

import (
	"io"

	"github.com/talostrading/sonic/internal"
)

type AsyncAdapter struct {
	ioc *IO
	fd  int
	pd  internal.PollData
	rw  io.ReadWriter
}

func NewAsyncAdapter(ioc *IO, rw io.ReadWriter, cb func(error, *AsyncAdapter)) {
	GetFd(rw, func(err error, fd int) {
		if err != nil {
			cb(err, nil)
		} else {
			f := &AsyncAdapter{
				fd:  fd,
				ioc: ioc,
				rw:  rw,
			}
			f.pd.Fd = fd
			cb(nil, f)
		}
	})
}

func (p *AsyncAdapter) AsyncRead(b []byte, cb AsyncCallback) {
	p.pd.Set(internal.ReadEvent, func(err error) {
		if err != nil {
			cb(err, 0)
		} else {
			n, err := p.rw.Read(b)
			cb(err, n)
		}
	})
	p.ioc.poller.SetRead(p.fd, &p.pd)
}

func (p *AsyncAdapter) AsyncWrite(b []byte, cb AsyncCallback) {
	p.pd.Set(internal.WriteEvent, func(err error) {
		if err != nil {
			cb(err, 0)
		} else {
			n, err := p.rw.Write(b)
			cb(err, n)
		}
	})
	p.ioc.poller.SetWrite(p.fd, &p.pd)
}

// TODO AsyncReadAll
// TODO AsyncWriteAll
