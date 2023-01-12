//go:build darwin || netbsd || freebsd || openbsd || dragonfly || linux

package internal

import (
	"fmt"
	"net"
	"reflect"
	"syscall"
)

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
		return nil
	case *net.UnixAddr:
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
