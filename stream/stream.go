package stream

import (
	"io"
	"net"
	"syscall"

	"github.com/talostrading/sonic"
	"github.com/talostrading/sonic/internal"
	"github.com/talostrading/sonic/sonicerrors"
)

// TODO PollData should be Slot
// TODO pendingReads/Writes should not be a map, but a slice indexed by fd.

type Stream struct {
	ioc        *sonic.IO
	fd         int
	localAddr  net.Addr
	remoteAddr net.Addr

	readReactor *readReactor
	slot        internal.PollData
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
	s.readReactor = newReadReactor(s)
	s.slot.Fd = fd
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

func (s *Stream) AsyncRead(b []byte, callback func(error, int)) {
	n, err := s.Read(b)

	if err == sonicerrors.ErrWouldBlock {
		if err := s.readReactor.prepare(b, callback); err != nil {
			callback(err, 0)
		}
	} else {
		callback(err, n)
	}
}

func (s *Stream) LocalAddr() net.Addr  { return s.localAddr }
func (s *Stream) RemoteAddr() net.Addr { return s.remoteAddr }
func (s *Stream) Close() error         { return syscall.Close(s.fd) }
