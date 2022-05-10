package sonic

import (
	"net"
	"time"

	"github.com/talostrading/sonic/internal"
)

var _ Stream = &stream{}

type stream struct {
	*file

	sock *internal.Socket
}

func Dial(ioc *IO, network, addr string) (Stream, error) {
	return DialTimeout(ioc, network, addr, 0)
}

func DialTimeout(ioc *IO, network, addr string, timeout time.Duration) (Stream, error) {
	sock, err := internal.NewSocket()
	if err != nil {
		return nil, err
	}

	err = sock.ConnectTimeout(network, addr, timeout)
	if err != nil {
		return nil, err
	}

	return &stream{
		file: &file{
			ioc: ioc,
			fd:  sock.Fd,
		},
	}, nil
}

func (s *stream) LocalAddr() net.Addr {
	return s.sock.LocalAddr
}
func (s *stream) RemoteAddr() net.Addr {
	return s.sock.RemoteAddr
}

func (s *stream) SetDeadline(t time.Time) error {
	// TODO
	return nil
}
func (s *stream) SetReadDeadline(t time.Time) error {
	// TODO
	return nil
}
func (s *stream) SetWriteDeadline(t time.Time) error {
	// TODO
	return nil
}
