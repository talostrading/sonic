package sonic

import (
	"fmt"
	"net"
	"os"
	"reflect"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

var _ Stream = &stream{}

type stream struct {
	*file

	localAddr  net.Addr
	remoteAddr net.Addr
}

func Dial(ioc *IO, network, addr string) (Stream, error) {
	return DialTimeout(ioc, network, addr, 0)
}

func DialTimeout(ioc *IO, network, addr string, timeout time.Duration) (Stream, error) {
	fd, localAddr, remoteAddr, err := connectTimeout(network, addr, timeout)
	if err != nil {
		return nil, err
	}
	return &stream{
		file: &file{
			ioc: ioc,
			fd:  fd,
		},
		localAddr:  localAddr,
		remoteAddr: remoteAddr,
	}, nil
}

func connectTimeout(network, addr string, timeout time.Duration) (int, net.Addr, net.Addr, error) {
	if timeout == 0 {
		timeout = time.Minute
	}

	switch network[:3] {
	case "tcp":
		remoteAddr, err := net.ResolveTCPAddr(network, addr)
		if err != nil {
			return -1, nil, nil, err
		}

		fd, err := createSocket(remoteAddr)
		if err != nil {
			return -1, nil, nil, err
		}

		if err := setSockOpt(fd); err != nil {
			return -1, nil, nil, err
		}

		err = syscall.Connect(fd, getTCPSockAddr(remoteAddr))
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
				return -1, nil, nil, ErrTimeout
			}

			_, err = syscall.GetsockoptInt(fd, syscall.SOL_SOCKET, syscall.SO_ERROR)
			if err != nil {
				syscall.Close(fd)
				return -1, nil, nil, os.NewSyscallError("getsockopt", err)
			}
		}

		localAddr, err := syscall.Getsockname(fd)
		return -1, fromSockAddr(localAddr), remoteAddr, err
	case "udp":
	default:
		// unix
	}

	return -1, nil, nil, nil
}

func (s *stream) LocalAddr() net.Addr {
	return s.localAddr
}
func (s *stream) RemoteAddr() net.Addr {
	return s.remoteAddr
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
		domain, typ = syscall.AF_UNSPEC, syscall.SOCK_DGRAM
		if len(addr.Zone) != 0 {
			domain = syscall.AF_INET6
		}
	case *net.UnixAddr:
		domain, typ = syscall.AF_UNIX, syscall.SOCK_STREAM // TODO maybe support udp for unix as well
	default:
		panic(fmt.Sprintf("unknown address type: %s", reflect.TypeOf(addr)))
	}

	fd, err := syscall.Socket(domain, typ, 0)
	if err != nil {
		return 0, os.NewSyscallError("socket", err)
	}

	return fd, nil
}

func getTCPSockAddr(addr *net.TCPAddr) syscall.Sockaddr {
	return &syscall.SockaddrInet4{
		Port: addr.Port,
		Addr: func() (b [4]byte) {
			copy(b[:], addr.IP.To4())
			return
		}(),
	}
}

func setSockOpt(fd int) error {
	err := syscall.SetNonblock(fd, true)
	if err != nil {
		err = os.NewSyscallError("set_nonblock", err)
	}
	// TODO probably no delay as well but after you write socket class
	return err
}

func fromSockAddr(sockAddr syscall.Sockaddr) net.Addr {
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
