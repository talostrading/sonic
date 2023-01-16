//go:build darwin || netbsd || freebsd || openbsd || dragonfly || linux

package internal

import (
	"fmt"
	"net"
	"reflect"
	"syscall"
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

func FromSockaddrTCP(sockAddr syscall.Sockaddr) net.Addr {
	switch addr := sockAddr.(type) {
	case *syscall.SockaddrInet4:
		return &net.TCPAddr{
			IP:   append([]byte{}, addr.Addr[:]...),
			Port: addr.Port,
		}
	case *syscall.SockaddrInet6:
		panic("not supported")
	default:
		panic("not supported")
	}
}

func FromSockaddrUDP(sockAddr syscall.Sockaddr) net.Addr {
	switch addr := sockAddr.(type) {
	case *syscall.SockaddrInet4:
		return &net.UDPAddr{
			IP:   append([]byte{}, addr.Addr[:]...),
			Port: addr.Port,
		}
	case *syscall.SockaddrInet6:
		panic("not supported")
	default:
		panic("not supported")
	}
}

func FromSockaddrUnix(sockAddr syscall.Sockaddr) net.Addr {
	panic("not supported")
}
