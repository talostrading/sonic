package websocket

import (
	"bytes"
	"testing"
)

// "I wanted to make sure the frame parser is robust against bad inputs"
func FuzzParseFrame(f *testing.F) {
	f.Add([]byte{0x81, 0x05, 0x48, 0x65, 0x6c, 0x6c, 0x6f}) // Valid Data

	f.Fuzz(func(t *testing.T, data []byte) {
		// Just see if it panics. If it crashes, you found a bug!
		ReadHeader(bytes.NewReader(data))
	})
}
