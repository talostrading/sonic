package websocket

import "io"

var zeroBytes = []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}

type Frame struct {
	header  []byte
	mask    []byte
	payload []byte
}

func NewFrame() *Frame {
	f := &Frame{}
	return f
}

func (f *Frame) Reset() {
	copy(f.header, zeroBytes)
	copy(f.mask, zeroBytes)
}

func (f *Frame) ReadFrom(r io.Reader) (n int64, err error) {
	return
}
