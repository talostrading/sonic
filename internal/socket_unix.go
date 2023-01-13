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

// Connect creates a socket capable of talking to the specified address.
//
// If network is:
// - tcp: Connect establishes a stream connection to the specified addr
// - udp: Connect creates a UDP socket and binds it to the specified address. If addr is empty, the socket is bound to a
//        random address by the kernel. The bound address will be returned as localAddr. remoteAddr will be nil.
// - unix: Connect establishes a stream connection to the specified addr
func Connect(
	network, addr string,
	opts ...sonicopts.Option,
) (fd int, localAddr, remoteAddr net.Addr, err error) {
	// TODO should provide
	if len(opts) > 0 {
		panic("cannot provide opts")
	}

	return ConnectTimeout(network, addr, 10*time.Second, opts...)
}

// ConnectTimeout connects with a timeout for all but UDP networks.
func ConnectTimeout(
	network, addr string,
	timeout time.Duration,
	opts ...sonicopts.Option,
) (fd int, localAddr, remoteAddr net.Addr, err error) {
	// TODO should provide
	if len(opts) > 0 {
		panic("cannot provide opts")
	}

	switch network[:3] {
	case "tcp":
		return ConnectTCP(network, addr, timeout, opts...)
	case "udp":
		return ConnectUDP(network, addr, opts...)
	case "uni":
		return -1, nil, nil, fmt.Errorf("unix domain not supported")
	default:
		return -1, nil, nil, errUnknownNetwork
	}
}

func ConnectTCP(
	network, addr string,
	timeout time.Duration,
	opts ...sonicopts.Option,
) (fd int, localAddr, remoteAddr net.Addr, err error) {
	// TODO should provide
	if len(opts) > 0 {
		panic("cannot provide opts")
	}

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

func ConnectUDP(
	network, addr string,
	opts ...sonicopts.Option,
) (fd int, localAddr, remoteAddr net.Addr, err error) {
	// TODO should provide
	if len(opts) > 0 {
		panic("cannot provide opts")
	}

	randomAddr := false
	if addr == "" {
		randomAddr = true
		localAddr = &net.UDPAddr{} // TODO handle ipv4 and ipv6 probably also need to modify CreateSocket with AF_UNSPEC
	} else {
		localAddr, err = net.ResolveUDPAddr(network, addr)
		if err != nil {
			return -1, nil, nil, err
		}
	}

	fd, err = CreateSocket(localAddr)
	if err != nil {
		return -1, nil, nil, err
	}

	if err := ApplyOpts(fd, sonicopts.Nonblocking(true)); err != nil {
		syscall.Close(fd)
		return -1, nil, nil, err
	}

	if err := syscall.Bind(fd, ToSockaddr(localAddr)); err != nil {
		syscall.Close(fd)
		return -1, nil, nil, err
	}

	if randomAddr {
		sockAddr, err := syscall.Getsockname(fd)
		if err != nil {
			return -1, nil, nil, err
		}
		localAddr = FromSockaddr(sockAddr)
	}

	return fd, localAddr, nil, nil
}

func Listen(network, addr string, opts ...sonicopts.Option) (fd int, listenAddr net.Addr, err error) {
	switch network[:3] {
	case "tcp":
		return ListenTCP(network, addr, opts...)
	case "udp":
		return ListenUDP(network, addr, opts...)
	case "uni":
		return -1, nil, fmt.Errorf("unix domain not supported")
	default:
		return -1, nil, errUnknownNetwork
	}
}

func ListenTCP(network, addr string, opts ...sonicopts.Option) (int, net.Addr, error) {
	localAddr, err := net.ResolveTCPAddr(network, addr)
	if err != nil {
		return -1, nil, err
	}

	fd, err := CreateSocket(localAddr)
	if err != nil {
		return -1, nil, err
	}

	if err := ApplyOpts(fd, opts...); err != nil {
		syscall.Close(fd)
		return -1, nil, err
	}

	if err := syscall.Bind(fd, ToSockaddr(localAddr)); err != nil {
		syscall.Close(fd)
		return -1, nil, os.NewSyscallError("bind", err)
	}

	if err := syscall.Listen(fd, ListenBacklog); err != nil {
		syscall.Close(fd)
		return -1, nil, os.NewSyscallError("listen", err)
	}

	return fd, localAddr, nil
}

func ListenUDP(network, addr string, opts ...sonicopts.Option) (int, net.Addr, error) {
	localAddr, err := net.ResolveUDPAddr(network, addr)
	if err != nil {
		return -1, nil, err
	}

	fd, err := CreateSocket(localAddr)
	if err != nil {
		return -1, nil, err
	}

	if err := ApplyOpts(fd, opts...); err != nil {
		syscall.Close(fd)
		return -1, nil, err
	}

	if err := syscall.Bind(fd, ToSockaddr(localAddr)); err != nil {
		syscall.Close(fd)
		return -1, nil, os.NewSyscallError("bind", err)
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
