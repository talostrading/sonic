package internal

import (
	"errors"
	"fmt"
	"github.com/talostrading/sonic/sonicerrors"
	"github.com/talostrading/sonic/sonicopts"
	"golang.org/x/sys/unix"
	"net"
	"os"
	"syscall"
	"time"
)

var (
	MaxListen int = 2048

	errUnknownNetwork = errors.New("unknown network argument")
)

func socket(domain, socketType, proto int) (fd int, err error) {
	fd, err = syscall.Socket(domain, socketType, proto)
	if err != nil {
		return -1, os.NewSyscallError("socket", err)
	}

	return fd, syscall.SetNonblock(fd, true) // queue up Mick Gordon
}

func CreateSocketTCP(network, addr string) (fd int, tcpAddr *net.TCPAddr, err error) {
	var (
		domain     = syscall.AF_INET
		socketType = syscall.SOCK_STREAM
	)
	if len(addr) > 0 {
		tcpAddr, err = net.ResolveTCPAddr(network, addr)
		if err != nil {
			return -1, nil, err
		}
		if len(tcpAddr.Zone) > 0 {
			domain = syscall.AF_INET6
		}
	}

	fd, err = socket(domain, socketType, 0)
	if err != nil {
		return -1, nil, err
	}

	return
}

func CreateSocketUDP(network, addr string) (fd int, udpAddr *net.UDPAddr, err error) {
	var (
		domain     = syscall.AF_INET
		socketType = syscall.SOCK_DGRAM
	)
	if len(addr) > 0 {
		udpAddr, err = net.ResolveUDPAddr(network, addr)
		if err != nil {
			return -1, nil, err
		}
		if len(udpAddr.Zone) > 0 {
			domain = syscall.AF_INET6
		}
	}

	fd, err = socket(domain, socketType, 0)
	if err != nil {
		return -1, nil, nil
	}

	return
}

// Connect connects to the specified endpoint. The created connection can be optionally bound to a local address
// by passing the option sonicopts.BindSocket(to net.Addr)
//
// If network is of type UDP, then addr is the address to which datagrams are sent by default, and the only address from
// which datagrams are received.
func Connect(
	network, addr string,
	opts ...sonicopts.Option,
) (fd int, localAddr, remoteAddr net.Addr, err error) {
	return ConnectTimeout(network, addr, 5*time.Second, opts...)
}

func ConnectTimeout(
	network, addr string,
	timeout time.Duration,
	opts ...sonicopts.Option,
) (fd int, localAddr, remoteAddr net.Addr, err error) {
	switch network[:3] {
	case "tcp":
		return ConnectTCP(network, addr, timeout, opts...)
	case "udp":
		return ConnectUDP(network, addr, timeout, opts...)
	case "uni":
		return -1, nil, nil, fmt.Errorf("unix domain not supported")
	default:
		return -1, nil, nil, errUnknownNetwork
	}
}

func connect(fd int, remoteAddr net.Addr, timeout time.Duration) error {
	if err := syscall.Connect(fd, ToSockaddr(remoteAddr)); err != nil {
		// this can happen if the socket is nonblocking, so we fix it with a select
		// https://man7.org/linux/man-pages/man2/connect.2.html#EINPROGRESS
		if err != syscall.EINPROGRESS && err != syscall.EAGAIN {
			return os.NewSyscallError("connect", err)
		}

		var fds unix.FdSet
		fds.Set(fd)

		t := unix.NsecToTimeval(timeout.Nanoseconds())

		n, err := unix.Select(fd+1, nil, &fds, nil, &t)
		if err != nil {
			return os.NewSyscallError("select", err)
		}

		if n == 0 {
			return sonicerrors.ErrTimeout
		}

		_, err = syscall.GetsockoptInt(fd, syscall.SOL_SOCKET, syscall.SO_ERROR)
		if err != nil {
			return os.NewSyscallError("getsockopt", err)
		}
	}

	return nil
}

func ConnectTCP(
	network, addr string,
	timeout time.Duration,
	opts ...sonicopts.Option,
) (fd int, localAddr, remoteAddr net.Addr, err error) {
	fd, remoteAddr, err = CreateSocketTCP(network, addr)
	if err != nil {
		return -1, nil, nil, err
	}

	if err = ApplyOpts(fd, opts...); err != nil {
		return -1, nil, nil, err
	}

	if err := connect(fd, remoteAddr, timeout); err != nil {
		return -1, nil, nil, err
	}

	localAddr, err = SocketAddressTCP(fd)
	return
}

func ConnectUDP(
	network, addr string,
	timeout time.Duration,
	opts ...sonicopts.Option,
) (fd int, localAddr, remoteAddr net.Addr, err error) {
	fd, remoteAddr, err = CreateSocketUDP(network, addr)
	if err != nil {
		return -1, nil, nil, err
	}

	if err := ApplyOpts(fd, opts...); err != nil {
		return -1, nil, nil, err
	}

	if err := connect(fd, remoteAddr, timeout); err != nil {
		return -1, nil, nil, err
	}

	localAddr, err = SocketAddressUDP(fd)
	return
}

func Listen(network, addr string, opts ...sonicopts.Option) (int, net.Addr, error) {
	// TODO unix domain as well
	if network[:3] != "tcp" && network[:3] != "uni" {
		return -1, nil, fmt.Errorf("network %s not supported", network[:3])
	}

	fd, localAddr, err := CreateSocketTCP(network, addr)
	if err != nil {
		return -1, nil, err
	}

	opts = sonicopts.AddOption(sonicopts.BindSocket(localAddr), opts)
	if err := ApplyOpts(fd, opts...); err != nil {
		return -1, nil, err
	}

	if err := syscall.Listen(fd, MaxListen); err != nil {
		syscall.Close(fd)
		return -1, nil, os.NewSyscallError("listen", err)
	}

	return fd, localAddr, nil
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
		case sonicopts.TypeBindSocket:
			addr := opt.Value().(net.Addr)
			return syscall.Bind(fd, ToSockaddr(addr))
		default:
			return fmt.Errorf("unsupported socket option %s", t)
		}
	}

	return nil
}

func SocketAddressTCP(fd int) (*net.TCPAddr, error) {
	addr, err := syscall.Getsockname(fd)
	if err != nil {
		return nil, err
	}
	return FromSockaddrTCP(addr), nil
}

func SocketAddressUDP(fd int) (*net.UDPAddr, error) {
	addr, err := syscall.Getsockname(fd)
	if err != nil {
		return nil, err
	}
	return FromSockaddrUDP(addr), nil
}
