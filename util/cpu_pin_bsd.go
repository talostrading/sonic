//go:build darwin || netbsd || freebsd || openbsd || dragonfly

package util

func PinTo(...int) error {
	return nil
}
