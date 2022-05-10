//go:build darwin || netbsd || freebsd || openbsd || dragonfly || linux

package internal

import (
	"fmt"
	"net"
	"os"
	"reflect"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

type Socket struct {
	Fd int

	LocalAddr  net.Addr
	RemoteAddr net.Addr

	// TODO opts... (nodelay, timeout, keepalive, timeout) in constructor
}

func NewSocket() (*Socket, error) {
	return &Socket{}, nil
}

func (s *Socket) ConnectTimeout(network, addr string, timeout time.Duration) error {
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
		panic(fmt.Sprintf("unsupported protocol: %s", network))
	}

	return nil
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

	if err := setDefaultOpts(fd); err != nil {
		return err
	}

	err = syscall.Connect(fd, toSockaddr(remoteAddr))
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
			return ErrTimeout
		}

		_, err = syscall.GetsockoptInt(fd, syscall.SOL_SOCKET, syscall.SO_ERROR)
		if err != nil {
			syscall.Close(fd)
			return os.NewSyscallError("getsockopt", err)
		}
	}

	sockAddr, err := syscall.Getsockname(fd)

	s.LocalAddr = fromSockaddr(sockAddr)
	s.RemoteAddr = remoteAddr
	s.Fd = fd

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
	err := syscall.SetNonblock(fd, true)
	if err != nil {
		err = os.NewSyscallError("set_nonblock", err)
	}

	err = syscall.SetsockoptInt(fd, syscall.IPPROTO_TCP, syscall.TCP_NODELAY, 1)
	if err != nil {
		err = os.NewSyscallError("tcp_no_delay", err)
	}

	return err
}

func toSockaddr(sockAddr net.Addr) syscall.Sockaddr {
	switch addr := sockAddr.(type) {
	case *net.TCPAddr:
		return &syscall.SockaddrInet4{
			Port: addr.Port,
			Addr: func() (b [4]byte) {
				copy(b[:], addr.IP.To4())
				return
			}(),
		}
	case *net.UDPAddr:
		return nil
	case *net.UnixAddr:
		return nil
	default:
		panic(fmt.Sprintf("unsupported address type: %s", reflect.TypeOf(sockAddr)))
	}
}

func fromSockaddr(sockAddr syscall.Sockaddr) net.Addr {
	switch addr := sockAddr.(type) {
	case *syscall.SockaddrInet4:
		return &net.TCPAddr{
			IP:   append([]byte{}, addr.Addr[:]...),
			Port: addr.Port,
		}
	case *syscall.SockaddrInet6:
		// TODO
	case *syscall.SockaddrUnix:
		return &net.UnixAddr{
			Name: addr.Name,
			Net:  "unix",
		}
	}

	return nil
}
