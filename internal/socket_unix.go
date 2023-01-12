package internal

import (
	"errors"
	"fmt"
	"github.com/talostrading/sonic/sonicerrors"
	"github.com/talostrading/sonic/sonicopts"
	"golang.org/x/sys/unix"
	"net"
	"os"
	"reflect"
	"syscall"
	"time"
)

var (
	ListenBacklog int = 2048

	errUnknownNetwork = errors.New("unknown network argument")
)

func CreateSocket(addr net.Addr) (int, error) {
	var (
		domain int
		typ    int
	)

	switch addr := addr.(type) {
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
		return -1, fmt.Errorf("unknown address type: %s", reflect.TypeOf(addr))
	}

	fd, err := syscall.Socket(domain, typ, 0)
	if err != nil {
		return 0, os.NewSyscallError("socket", err)
	}

	return fd, nil
}

func Connect(network, addr string) (fd int, localAddr, remoteAddr net.Addr, err error) {
	return ConnectTimeout(network, addr, 10*time.Second)
}

func ConnectTimeout(network, addr string, timeout time.Duration) (fd int, localAddr, remoteAddr net.Addr, err error) {
	switch network[:3] {
	case "tcp":
		return ConnectTCP(network, addr, timeout)
	case "udp":
		return -1, nil, nil, fmt.Errorf("udp not supported")
	case "uni":
		return -1, nil, nil, fmt.Errorf("unix domain not supported")
	default:
		return -1, nil, nil, errUnknownNetwork
	}
}

func ConnectTCP(network, addr string, timeout time.Duration) (fd int, localAddr, remoteAddr net.Addr, err error) {
	remoteAddr, err = net.ResolveTCPAddr(network, addr)
	if err != nil {
		return -1, nil, nil, err
	}

	fd, err = CreateSocket(remoteAddr)
	if err != nil {
		return -1, nil, nil, err
	}

	// TODO make nodelay optional
	err = ApplyOpts(fd, sonicopts.Nonblocking(true), sonicopts.NoDelay(true))
	if err != nil {
		syscall.Close(fd)
		return -1, nil, nil, err
	}

	err = syscall.Connect(fd, ToSockaddr(remoteAddr))
	if err != nil {
		// this can happen if the socket is nonblocking, so we fix it with a select
		// https://man7.org/linux/man-pages/man2/connect.2.html#EINPROGRESS
		if err != syscall.EINPROGRESS && err != syscall.EAGAIN {
			syscall.Close(fd)
			return -1, nil, nil, os.NewSyscallError("connect", err)
		}

		var fds unix.FdSet
		fds.Set(fd)

		t := unix.NsecToTimeval(timeout.Nanoseconds())

		n, err := unix.Select(fd+1, nil, &fds, nil, &t)
		if err != nil {
			syscall.Close(fd)
			return -1, nil, nil, os.NewSyscallError("select", err)
		}

		if n == 0 {
			syscall.Close(fd)
			return -1, nil, nil, sonicerrors.ErrTimeout
		}

		_, err = syscall.GetsockoptInt(fd, syscall.SOL_SOCKET, syscall.SO_ERROR)
		if err != nil {
			syscall.Close(fd)
			return -1, nil, nil, os.NewSyscallError("getsockopt", err)
		}
	}

	localAddr, err = SocketAddress(fd)

	return
}

func Listen(network, addr string, opts ...sonicopts.Option) (fd int, err error) {
	switch network[:3] {
	case "tcp":
		return ListenTCP(network, addr, opts...)
	case "udp":
		return -1, fmt.Errorf("udp not supported")
	case "uni":
		return -1, fmt.Errorf("unix domain not supported")
	default:
		return -1, errUnknownNetwork
	}
}

func ListenTCP(network, addr string, opts ...sonicopts.Option) (fd int, err error) {
	localAddr, err := net.ResolveTCPAddr(network, addr)
	if err != nil {
		return -1, err
	}

	fd, err = CreateSocket(localAddr)
	if err != nil {
		return -1, err
	}

	if err := ApplyOpts(fd, opts...); err != nil {
		syscall.Close(fd)
		return -1, err
	}

	if err := syscall.Bind(fd, ToSockaddr(localAddr)); err != nil {
		syscall.Close(fd)
		return -1, os.NewSyscallError("bind", err)
	}

	if err := syscall.Listen(fd, ListenBacklog); err != nil {
		syscall.Close(fd)
		return -1, os.NewSyscallError("listen", err)
	}

	return fd, nil
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

func SocketAddress(fd int) (net.Addr, error) {
	addr, err := syscall.Getsockname(fd)
	if err != nil {
		return nil, err
	}
	return FromSockaddr(addr), nil
}
