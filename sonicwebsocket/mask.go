package sonicwebsocket

import (
	"crypto/rand"
)

func mask(mask, b []byte) {
	for i := range b {
		b[i] ^= mask[i&3]
	}
}

func genMask(b []byte) {
	rand.Read(b)
}
