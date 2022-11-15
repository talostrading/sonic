//go:build darwin || netbsd || freebsd || openbsd || dragonfly || linux

package internal

import (
	"fmt"
	"net"
	"os"
	"reflect"
	"syscall"
	"time"

	"github.com/talostrading/sonic/sonicerrors"
	"github.com/talostrading/sonic/sonicopts"
	"golang.org/x/sys/unix"
)

type Socket struct {
	poller Poller
	opts   []sonicopts.Option

	Fd         int
	pd         PollData
	LocalAddr  net.Addr
	RemoteAddr net.Addr
}

func NewSocket(poller Poller, opts ...sonicopts.Option) (*Socket, error) {
	s := &Socket{
		poller: poller,
		opts:   opts,

		Fd: -1,
	}

	return s, nil
}

func (s *Socket) cleanup() (err error) {
	if s.Fd > -1 {
		s.Fd = -1
		err = syscall.Close(s.Fd)
	}
	s.pd.Clear()
	return
}

func (s *Socket) ConnectTimeout(
	network,
	addr string,
	timeout time.Duration,
) error {
	if timeout == 0 {
		timeout = time.Minute // TODO this should be in the wrapper
	}

	switch network[:3] {
	case "tcp":
		return s.connectTCP(network, addr, timeout)
	case "udp":
		return s.connectUDP(network, addr, timeout)
	case "uni":
		return s.connectUnix(network, addr, timeout)
	default:
		return fmt.Errorf("unsupported protocol: %s", network)
	}
}

func (s *Socket) Listen(network, addr string) error {
	switch network[:3] {
	case "tcp":
		return s.listenTCP(network, addr)
	case "uni":
		return s.listenUnix(network, addr)
	default:
		return fmt.Errorf("unsupported protocol: %s", network)
	}
}

func (s *Socket) listenTCP(network, addr string) error {
	localAddr, err := net.ResolveTCPAddr(network, addr)
	if err != nil {
		s.cleanup()
		return err
	}

	fd, err := createSocket(localAddr)
	if err != nil {
		s.cleanup()
		return err
	}

	s.Fd = fd
	s.LocalAddr = localAddr

	if err := ApplyOpts(s.Fd, s.opts...); err != nil {
		s.cleanup()
		return err
	}

	sockAddr := ToSockaddr(localAddr)
	if err := syscall.Bind(fd, sockAddr); err != nil {
		s.cleanup()
		return os.NewSyscallError("bind", err)
	}

	if err := syscall.Listen(fd, 2048); err != nil {
		s.cleanup()
		return os.NewSyscallError("listen", err)
	}

	return nil
}

func (s *Socket) listenUnix(network, addr string) error {
	panic("cannot listen on unix sockets atm")
}

func (s *Socket) prepareTCP(network, addr string) error {
	remoteAddr, err := net.ResolveTCPAddr(network, addr)
	if err != nil {
		s.cleanup()
		return err
	}

	fd, err := createSocket(remoteAddr)
	if err != nil {
		s.cleanup()
		return err
	}
	s.Fd = fd
	s.pd.Fd = fd
	s.RemoteAddr = remoteAddr

	if err := ApplyOpts(s.Fd, s.opts...); err != nil {
		s.cleanup()
		return err
	}

	return nil
}

// TODO test this
func (s *Socket) asyncConnectTCP(network, addr string, cb func(error)) {
	s.opts = sonicopts.Add(s.opts, sonicopts.Nonblocking(true))

	if err := s.prepareTCP(network, addr); err != nil {
		cb(err)
		s.cleanup()
		return
	}

	err := syscall.Connect(s.Fd, ToSockaddr(s.RemoteAddr))
	if err != nil {
		if err != syscall.EINPROGRESS && err != syscall.EAGAIN {
			s.cleanup()
			cb(os.NewSyscallError("connect", err))
			return
		}

		// If the socket is non-blocking and the connection cannot be made immediately, connect fails with EINPROGRESS
		// or EAGAIN. We poll for connect readiness with the poller.
		// https://man7.org/linux/man-pages/man2/connect.2.html#EINPROGRESS

		s.pd.Set(WriteEvent, func(err error) {
			if err == nil {
				_, err = syscall.GetsockoptInt(s.Fd, syscall.SOL_SOCKET, syscall.SO_ERROR)
			}

			if err != nil {
				s.cleanup()
			} else {
				var localAddr syscall.Sockaddr
				localAddr, err = syscall.Getsockname(s.Fd)
				s.LocalAddr = FromSockaddr(localAddr)
			}
			cb(err)
		})

		err = s.poller.SetWrite(s.Fd, &s.pd)
		if err != nil {
			s.cleanup()
			cb(err)
			return
		}
	}
}

func (s *Socket) connectTCP(network, addr string, timeout time.Duration) error {
	if err := s.prepareTCP(network, addr); err != nil {
		return err
	}

	err := syscall.Connect(s.Fd, ToSockaddr(s.RemoteAddr))
	if err != nil {
		if err != syscall.EINPROGRESS && err != syscall.EAGAIN {
			s.cleanup()
			return os.NewSyscallError("connect", err)
		}

		// If the socket is non-blocking and the connection cannot be made immediately, connect fails with EINPROGRESS
		// or EAGAIN. We poll for connect readiness with the specified timeout and wait.
		// https://man7.org/linux/man-pages/man2/connect.2.html#EINPROGRESS

		var fds unix.FdSet
		fds.Set(s.Fd)

		t := unix.NsecToTimeval(timeout.Nanoseconds())

		n, err := unix.Select(s.Fd+1, nil, &fds, nil, &t)
		if err != nil {
			s.cleanup()
			return os.NewSyscallError("select", err)
		}

		if n == 0 {
			s.cleanup()
			return sonicerrors.ErrTimeout
		}

		_, err = syscall.GetsockoptInt(s.Fd, syscall.SOL_SOCKET, syscall.SO_ERROR)
		if err != nil {
			s.cleanup()
			return os.NewSyscallError("getsockopt", err)
		}
	}

	sockAddr, err := syscall.Getsockname(s.Fd)
	s.LocalAddr = FromSockaddr(sockAddr)

	return nil
}

func (s *Socket) connectUDP(network, addr string, timeout time.Duration) error {
	// TODO
	return nil
}

func (s *Socket) connectUnix(network, addr string, timeout time.Duration) error {
	// TODO
	return nil
}

func createSocket(resolvedAddr net.Addr) (int, error) {
	var (
		domain int
		typ    int
	)

	switch addr := resolvedAddr.(type) {
	case *net.TCPAddr:
		domain, typ = syscall.AF_INET, syscall.SOCK_STREAM
		if len(addr.Zone) != 0 {
			domain = syscall.AF_INET6
		}
	case *net.UDPAddr:
		domain, typ = syscall.AF_INET, syscall.SOCK_DGRAM
		if len(addr.Zone) != 0 {
			domain = syscall.AF_INET6
		}
	case *net.UnixAddr:
		domain, typ = syscall.AF_UNIX, syscall.SOCK_STREAM
	default:
		panic(fmt.Sprintf("unknown address type: %s", reflect.TypeOf(addr)))
	}

	fd, err := syscall.Socket(domain, typ, 0)
	if err != nil {
		return 0, os.NewSyscallError("socket", err)
	}

	return fd, nil
}
