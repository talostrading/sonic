//go:build darwin || netbsd || freebsd || openbsd || dragonfly

package ipv4

import (
	"github.com/talostrading/sonic"
)

func SetMulticastAll(socket *sonic.Socket, all bool) error {
	// BSD makes more sense here. See peer_ipv4_linux_test.go for an explanation.
	return nil
}

func GetMulticastAll(socket *sonic.Socket) (bool, error) {
	return false, nil
}
