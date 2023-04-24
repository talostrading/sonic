//go:build darwin || netbsd || freebsd || openbsd || dragonfly

package ipv4

import (
	"github.com/talostrading/sonic"
)

func SetMulticastAll(sock *sonic.Socket, all bool) error {
	return nil
}
