package websocket

import (
	"encoding/binary"
	"io"
	"sync"

	"github.com/talostrading/sonic/util"
)

var zeroBytes = []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}

type Frame struct {
	header  []byte
	mask    []byte
	payload []byte
}

var (
	_ io.ReaderFrom = &Frame{}
	_ io.WriterTo   = &Frame{}
)

func NewFrame() *Frame {
	f := &Frame{
		header:  make([]byte, 10),
		mask:    make([]byte, 4),
		payload: make([]byte, 0, 1024),
	}
	return f
}

func (f *Frame) Reset() {
	copy(f.header, zeroBytes)
	copy(f.mask, zeroBytes)
	f.payload = f.payload[:0]
}

func (f *Frame) ExtraHeaderLen() (n int) {
	switch f.header[1] & 127 {
	case 127:
		n = 8
	case 126:
		n = 2
	}
	return
}

func (f *Frame) PayloadLength() int {
	length := uint64(f.header[1] & 127)
	switch length {
	case 126:
		length = uint64(binary.BigEndian.Uint16(f.header[2:]))
	case 127:
		length = binary.BigEndian.Uint64(f.header[2:])
	}
	return int(length)
}

func (f *Frame) SetPayloadLength() (bytes int) {
	n := len(f.payload)

	switch {
	case n > 65535: // more than two bytes needed for extra length
		bytes = 8 //nolint:ineffassign
		f.header[1] |= uint8(127)
		binary.BigEndian.PutUint64(f.header[2:], uint64(n))
	case n > 125:
		bytes = 2 //nolint:ineffassign
		f.header[1] |= uint8(126)
		binary.BigEndian.PutUint16(f.header[2:], uint16(n))
		return 2
	default:
		bytes = 0 //nolint:ineffassign
		f.header[1] |= uint8(n)
	}
	return
}

// An unfragmented message consists of a single frame with the FIN bit set and an opcode other than 0.
//
// A fragmented message consists of a single frame with the FIN bit clear and an opcode other than 0, followed by zero
// or more frames with the FIN bit clear and the opcode set to 0, and terminated by a single frame with the FIN bit set
// and an opcode of 0.
func (f *Frame) IsFIN() bool {
	return f.header[0]&bitFIN != 0
}

func (f *Frame) IsRSV1() bool {
	return f.header[0]&bitRSV1 != 0
}

func (f *Frame) IsRSV2() bool {
	return f.header[0]&bitRSV2 != 0
}

func (f *Frame) IsRSV3() bool {
	return f.header[0]&bitRSV3 != 0
}

func (f *Frame) Opcode() Opcode {
	return Opcode(f.header[0] & 15)
}

func (f *Frame) IsMasked() bool {
	return f.header[1]&bitIsMasked != 0
}

func (f *Frame) SetFIN() {
	f.header[0] |= bitFIN
}

func (f *Frame) SetRSV1() {
	f.header[0] |= bitRSV1
}

func (f *Frame) SetRSV2() {
	f.header[0] |= bitRSV2
}

func (f *Frame) SetRSV3() {
	f.header[0] |= bitRSV3
}

func (f *Frame) SetOpcode(c Opcode) {
	c &= 15
	f.header[0] &= 15 << 4
	f.header[0] |= uint8(c)
}

func (f *Frame) SetContinuation() {
	f.SetOpcode(OpcodeContinuation)
}

func (f *Frame) SetText() {
	f.SetOpcode(OpcodeText)
}

func (f *Frame) SetBinary() {
	f.SetOpcode(OpcodeBinary)
}

func (f *Frame) SetClose() {
	f.SetOpcode(OpcodeClose)
}

func (f *Frame) SetPing() {
	f.SetOpcode(OpcodePing)
}

func (f *Frame) SetPong() {
	f.SetOpcode(OpcodePong)
}

func (f *Frame) SetPayload(b []byte) {
	f.payload = append(f.payload[:0], b...)
}

func (f *Frame) MaskKey() []byte {
	return f.mask[:]
}

func (f *Frame) Payload() []byte {
	return f.payload
}

func (f *Frame) Mask() {
	f.header[1] |= bitIsMasked
	GenMask(f.mask[:])
	if len(f.payload) > 0 {
		Mask(f.mask[:], f.payload)
	}
}

func (f *Frame) Unmask() {
	if len(f.payload) > 0 {
		key := f.MaskKey()
		Mask(key, f.payload)
	}
	f.header[1] ^= bitIsMasked
}

func (f *Frame) ReadFrom(r io.Reader) (nt int64, err error) {
	var n int
	n, err = io.ReadFull(r, f.header[:2])
	nt += int64(n)

	if err == nil {
		m := f.ExtraHeaderLen()
		if m > 0 {
			n, err = io.ReadFull(r, f.header[2:m+2])
			nt += int64(n)
		}

		if err == nil && f.IsMasked() {
			n, err = io.ReadFull(r, f.mask[:4])
			nt += int64(n)
		}

		if err == nil {
			if pn := f.PayloadLength(); pn > 0 {
				if pn > MaxMessageSize {
					err = ErrPayloadTooBig
				} else {
					f.payload = util.ExtendSlice(f.payload, pn)
					n, err = io.ReadFull(r, f.payload[:pn])
					nt += int64(n)
				}
			} else if pn == 0 {
				f.payload = f.payload[:0]
			}
		}
	}

	return
}

func (f *Frame) WriteTo(w io.Writer) (n int64, err error) {
	var nn int

	nn, err = w.Write(f.header[:2+f.SetPayloadLength()])
	n += int64(nn)

	if err == nil {
		if f.IsMasked() {
			nn, err = w.Write(f.mask[:])
			n += int64(nn)
		}

		if err == nil && f.PayloadLength() > 0 {
			nn, err = w.Write(f.payload[:f.PayloadLength()])
			n += int64(nn)
		}
	}

	return
}

var framePool = sync.Pool{
	New: func() interface{} {
		return NewFrame()
	},
}

func AcquireFrame() *Frame {
	return framePool.Get().(*Frame)
}

func ReleaseFrame(f *Frame) {
	f.Reset()
	framePool.Put(f)
}
