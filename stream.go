package sonic

import (
	"fmt"
	"net"
	"os"
	"reflect"
	"syscall"
	"time"
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

		//if err := syscall.SetNonblock(fd, true); err != nil {
		//return os.NewSyscallError("set_nonblock", err)
		//}

		err = syscall.Connect(fd, getTCPSockAddr(remoteAddr))
		if err != nil {
			// this can happen if the socket is nonblocking
			if err != syscall.EINPROGRESS || err != syscall.EAGAIN {
				syscall.Close(fd)
				return -1, nil, nil, os.NewSyscallError("connect", err)
			}
			// TODO gotta handle this from: https://man7.org/linux/man-pages/man2/connect.2.html
			/*
							EINPROGRESS
				              The socket is nonblocking and the connection cannot be
				              completed immediately.  (UNIX domain sockets failed with
				              EAGAIN instead.)  It is possible to select(2) or poll(2)
				              for completion by selecting the socket for writing.  After
				              select(2) indicates writability, use getsockopt(2) to read
				              the SO_ERROR option at level SOL_SOCKET to determine
				              whether connect() completed successfully (SO_ERROR is
				              zero) or unsuccessfully (SO_ERROR is one of the usual
				              error codes listed here, explaining the reason for the
				              failure).
			*/
			return -1, nil, nil, err
		}

		localAddr, err := syscall.Getsockname(fd)
		fmt.Println("local", localAddr)
		fmt.Println("remote", remoteAddr)
		return -1, nil, nil, err
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
