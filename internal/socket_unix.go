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
	Fd int

	LocalAddr  net.Addr
	RemoteAddr net.Addr

	opts []sonicopts.Option
}

func NewSocket(opts ...sonicopts.Option) (*Socket, error) {
	s := &Socket{opts: opts}

	return s, nil
}

// TODO AsyncConnect
// steps: (requires non blocking socket)
// 1. res := connect()
// 2. if res <  res != EINPROGRESS, fail and close socket
// 3. if res == 0 then we connected
// 3. otherwise you have to wait for the socket to be writable
//https://stackoverflow.com/questions/10187347/async-connect-and-disconnect-with-epoll-linux

func (s *Socket) ConnectTimeout(
	network,
	addr string,
	timeout time.Duration,
) error {
	if timeout == 0 {
		timeout = time.Minute
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

func ApplyOpts(fd int, opts ...sonicopts.Option) error {
	for _, opt := range opts {
		switch t := opt.Type(); t {
		case sonicopts.TypeNonblocking:
			v := opt.Value().(bool)
			if err := syscall.SetNonblock(fd, v); err != nil {
				return os.NewSyscallError(fmt.Sprintf("set_nonblock(%v)", v), err)
			}
		case sonicopts.TypeReusePort:
			v := opt.Value().(bool)

			iv := 0
			if v {
				iv = 1
			}

			if err := syscall.SetsockoptInt(
				fd,
				syscall.SOL_SOCKET,
				unix.SO_REUSEPORT,
				iv,
			); err != nil {
				return os.NewSyscallError(fmt.Sprintf("reuse_port(%v)", v), err)
			}
		case sonicopts.TypeReuseAddr:
			v := opt.Value().(bool)

			iv := 0
			if v {
				iv = 1
			}

			if err := syscall.SetsockoptInt(
				fd,
				syscall.SOL_SOCKET,
				unix.SO_REUSEADDR,
				iv,
			); err != nil {
				return os.NewSyscallError(fmt.Sprintf("reuse_address(%v)", v), err)
			}
		case sonicopts.TypeNoDelay:
			v := opt.Value().(bool)
			iv := 0
			if v {
				iv = 1
			}

			if err := syscall.SetsockoptInt(
				fd,
				syscall.IPPROTO_TCP,
				syscall.TCP_NODELAY,
				iv,
			); err != nil {
				return os.NewSyscallError(fmt.Sprintf("tcp_no_delay(%v)", v), err)
			}
		default:
			return fmt.Errorf("unsupported socket option %s", t)
		}
	}

	return nil
}

func (s *Socket) listenTCP(network, addr string) error {
	localAddr, err := net.ResolveTCPAddr(network, addr)
	if err != nil {
		return err
	}

	fd, err := createSocket(localAddr)
	if err != nil {
		return err
	}

	s.Fd = fd
	s.LocalAddr = localAddr

	if err := ApplyOpts(s.Fd, s.opts...); err != nil {
		return err
	}

	sockAddr := ToSockaddr(localAddr)
	if err := syscall.Bind(fd, sockAddr); err != nil {
		syscall.Close(fd)
		return os.NewSyscallError("bind", err)
	}

	if err := syscall.Listen(fd, 2048); err != nil {
		syscall.Close(fd)
		return os.NewSyscallError("listen", err)
	}

	return nil
}

func (s *Socket) listenUnix(network, addr string) error {
	panic("cannot listen on unix sockets atm")
}

func (s *Socket) connectTCP(network, addr string, timeout time.Duration) error {
	remoteAddr, err := net.ResolveTCPAddr(network, addr)
	if err != nil {
		return err
	}

	fd, err := createSocket(remoteAddr)
	if err != nil {
		return err
	}
	s.Fd = fd

	// TODO get rid of this once you also support blocking sockets
	// this means having two implementations of file: blocking and nonblocking
	if err := setDefaultOpts(fd); err != nil {
		return err
	}

	if err := ApplyOpts(s.Fd, s.opts...); err != nil {
		return err
	}

	err = syscall.Connect(fd, ToSockaddr(remoteAddr))
	if err != nil {
		// this can happen if the socket is nonblocking, so we fix it with a select
		// https://man7.org/linux/man-pages/man2/connect.2.html#EINPROGRESS
		if err != syscall.EINPROGRESS && err != syscall.EAGAIN {
			syscall.Close(fd)
			return os.NewSyscallError("connect", err)
		}

		var fds unix.FdSet
		fds.Set(fd)

		t := unix.NsecToTimeval(timeout.Nanoseconds())

		n, err := unix.Select(fd+1, nil, &fds, nil, &t)
		if err != nil {
			syscall.Close(fd)
			return os.NewSyscallError("select", err)
		}

		if n == 0 {
			return sonicerrors.ErrTimeout
		}

		_, err = syscall.GetsockoptInt(fd, syscall.SOL_SOCKET, syscall.SO_ERROR)
		if err != nil {
			syscall.Close(fd)
			return os.NewSyscallError("getsockopt", err)
		}
	}

	sockAddr, err := syscall.Getsockname(fd)

	s.LocalAddr = FromSockaddr(sockAddr)
	s.RemoteAddr = remoteAddr

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

func setDefaultOpts(fd int) error {
	if err := syscall.SetNonblock(fd, true); err != nil {
		return os.NewSyscallError("set_nonblock(true)", err)
	}

	if err := syscall.SetsockoptInt(fd, syscall.IPPROTO_TCP, syscall.TCP_NODELAY, 1); err != nil {
		return os.NewSyscallError("tcp_no_delay", err)
	}

	return nil
}
