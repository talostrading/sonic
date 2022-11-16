package sonic

import (
	"github.com/talostrading/sonic/internal"
	"github.com/talostrading/sonic/sonicopts"
	"net"
)

// TODO callbacks: OnSocketCreate, OnSocketBound, OnSocketConnect, OnSocketError
// TODO for a timeout on an async connect, just have a timer
// TODO is the socket IP then set the number of TCP SYN retransmits we send before aborting the connect

var _ Socket = &socket{}

type socket struct {
	FileDescriptor

	ioc  *IO
	opts []sonicopts.Option

	base *internal.Socket
}

func Connect(ioc *IO, network, addr string, opts ...sonicopts.Option) (Socket, error) {
	s := &socket{
		ioc:  ioc,
		opts: opts,
	}

	var err error

	s.base, err = internal.NewSocket(ioc.poller, opts...)
	if err != nil {
		return nil, err
	}

	err = s.base.ConnectTimeout(network, addr, 0)
	if err != nil {
		return nil, err
	}

	s.FileDescriptor, err = NewFileDescriptor(ioc, s.base.Fd, opts...)
	if err != nil {
		return nil, err
	}

	return s, nil
}

func (s *socket) RecvMsg() ([]byte, error) {
	// TODO this should be in internal.Socket
	return nil, nil
}

func (s *socket) Opts() []sonicopts.Option {
	return nil
}

func (s *socket) SetOpts(opts ...sonicopts.Option) {
	s.opts = sonicopts.Add(s.opts, opts...)
}

func (s *socket) RemoteAddr() net.Addr {
	return s.base.RemoteAddr
}

func (s *socket) LocalAddr() net.Addr {
	return s.base.LocalAddr
}
