package stream

import (
	"io"
	"net"
	"syscall"

	"github.com/talostrading/sonic"
	"github.com/talostrading/sonic/internal"
	"github.com/talostrading/sonic/sonicerrors"
)

type Stream struct {
	ioc        *sonic.IO
	fd         int
	localAddr  net.Addr
	remoteAddr net.Addr
}

func Connect(ioc *sonic.IO, transport, addr string) (*Stream, error) {
	fd, localAddr, remoteAddr, err := internal.Connect(transport, addr)
	if err != nil {
		return nil, err
	}
	return newStream(
		ioc,
		fd,
		localAddr,
		remoteAddr,
	)
}

func newStream(
	ioc *sonic.IO,
	fd int,
	localAddr net.Addr,
	remoteAddr net.Addr,
) (*Stream, error) {
	s := &Stream{
		ioc:        ioc,
		fd:         fd,
		localAddr:  localAddr,
		remoteAddr: remoteAddr,
	}
	return s, nil
}

func (s *Stream) Read(b []byte) (n int, err error) {
	n, err = syscall.Read(s.fd, b)
	if err != nil {
		if err == syscall.EWOULDBLOCK || err == syscall.EAGAIN {
			return 0, sonicerrors.ErrWouldBlock
		}
		return 0, err
	}
	if n == 0 {
		return 0, io.EOF
	}
	return n, nil
}

func (s *Stream) LocalAddr() net.Addr  { return s.localAddr }
func (s *Stream) RemoteAddr() net.Addr { return s.remoteAddr }
