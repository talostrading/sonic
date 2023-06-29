//go:build darwin || netbsd || freebsd || openbsd || dragonfly || linux

package internal

import (
	"fmt"
	"net"
	"reflect"
	"syscall"

	"github.com/talostrading/sonic/util"
	"golang.org/x/sys/unix"
)

// TODO Handle IPv6

func ToSockaddr(addr net.Addr) syscall.Sockaddr {
	switch addr := addr.(type) {
	case *net.TCPAddr:
		return &syscall.SockaddrInet4{
			Port: addr.Port,
			Addr: func() (b [4]byte) {
				copy(b[:], addr.IP.To4())
				return
			}(),
		}
	case *net.UDPAddr:
		return &syscall.SockaddrInet4{
			Port: addr.Port,
			Addr: func() (b [4]byte) {
				copy(b[:], addr.IP.To4())
				return
			}(),
		}
	case *net.UnixAddr:
		panic("unix not supported")
		return nil
	default:
		panic(fmt.Sprintf("unsupported address type: %s", reflect.TypeOf(addr)))
	}
}

func FromSockaddr(sockAddr syscall.Sockaddr) net.Addr {
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

func IsNonblocking(fd int) (bool, error) {
	v, err := unix.FcntlInt(uintptr(fd), unix.F_GETFL, 0)
	if err != nil {
		return false, err
	}
	return v&unix.O_NONBLOCK == unix.O_NONBLOCK, nil
}

func IsNoDelay(fd int) (bool, error) {
	v, err := syscall.GetsockoptInt(fd, syscall.IPPROTO_TCP, syscall.TCP_NODELAY)
	if err != nil {
		return false, err
	}
	return v&syscall.TCP_NODELAY == syscall.TCP_NODELAY, nil
}

func FromSockaddrUDP(sockAddr syscall.Sockaddr, to *net.UDPAddr) *net.UDPAddr {
	switch addr := sockAddr.(type) {
	case *syscall.SockaddrInet4:
		to.IP = util.ExtendSlice(to.IP, net.IPv4len)
		copy(to.IP, addr.Addr[:])
		to.Port = addr.Port
	case *syscall.SockaddrInet6:
		to.IP = util.ExtendSlice(to.IP, net.IPv6len)
		copy(to.IP, addr.Addr[:])
		to.Port = addr.Port
		// TODO zoneID (not sure the encoding)
	default:
		panic("not supported")
	}
	return to
}
