package sonicwebsocket

import (
	"encoding/binary"
	"fmt"
	"io"
	"sync"

	"github.com/talostrading/sonic/util"
)

var (
	zeroBytes = func() []byte {
		b := make([]byte, 10)
		for i := range b {
			b[i] = 0
		}
		return b
	}()
)

type Frame struct {
	header  []byte
	mask    []byte
	payload []byte
}

func NewFrame() *Frame {
	return &Frame{
		header:  make([]byte, 10),
		mask:    make([]byte, 4),
		payload: make([]byte, DefaultFramePayloadSize),
	}
}

func (fr *Frame) ReadFrom(r io.Reader) (int64, error) {
	var n int
	var err error

	n, err = io.ReadFull(r, fr.header[:2])
	if err == io.ErrUnexpectedEOF {
		return int64(n), ErrReadingHeader
	}

	if err == nil {
		m := fr.readMore()
		if m > 0 {
			n, err = io.ReadFull(r, fr.header[2:m+2])
			if err == io.ErrUnexpectedEOF {
				err = ErrReadingExtendedLength
			}
		}

		if err == nil && fr.IsMasked() {
			n, err = io.ReadFull(r, fr.mask[:4])
			if err == io.ErrUnexpectedEOF {
				err = ErrReadingMask
			}
		}

		if err == nil {
			if payloadLen := fr.Len(); payloadLen > MaxFramePayloadSize {
				return int64(n), ErrPayloadTooBig
			} else if payloadLen > 0 {
				nn := int(payloadLen)
				if nn < 0 {
					panic("invalid uint64 to int conversion")
				}

				if nn > 0 {
					fr.payload = util.ExtendByteSlice(fr.payload, nn)
					n, err = io.ReadFull(r, fr.payload)
				}
			}
		}
	}

	return int64(n), err
}

func (fr *Frame) WriteTo(w io.Writer) (n int64, err error) {
	var nn int

	nn, err = w.Write(fr.header[:2+fr.payloadBytes()])
	n += int64(nn)

	if err == nil {
		if fr.IsMasked() {
			nn, err = w.Write(fr.mask)
			n += int64(nn)
		}

		if err == nil && len(fr.payload) > 0 {
			nn, err = w.Write(fr.payload)
			n += int64(nn)
		}
	}

	return
}

func (fr *Frame) payloadBytes() int {
	n := len(fr.payload)

	switch {
	case n > 65535: // more than two bytes needed
		fr.header[1] |= uint8(127)
		binary.BigEndian.PutUint64(fr.header[2:], uint64(n))
		return 8
	case n > 125:
		fr.header[1] |= uint8(125)
		binary.BigEndian.PutUint16(fr.header[2:], uint16(n))
		return 2
	default:
		fr.header[1] |= uint8(n)
		return 0
	}
}

func (fr *Frame) readMore() (n int) {
	switch fr.header[1] & 127 {
	case 127:
		n = 8
	case 126:
		n = 2
	}
	return
}

func (fr *Frame) Reset() {
	copy(fr.header, zeroBytes)
	copy(fr.mask, zeroBytes)
}

func (fr *Frame) IsFin() bool {
	return fr.header[0]&finBit != 0
}

func (fr *Frame) IsRSV1() bool {
	return fr.header[0]&rsv1Bit != 0
}

func (fr *Frame) IsRSV2() bool {
	return fr.header[0]&rsv2Bit != 0
}

func (fr *Frame) IsRSV3() bool {
	return fr.header[0]&rsv3Bit != 0
}

func (fr *Frame) Opcode() Opcode {
	return Opcode(fr.header[0] & 15)
}

func (fr *Frame) IsContinuation() bool {
	return fr.Opcode() == OpcodeContinuation
}

func (fr *Frame) IsText() bool {
	return fr.Opcode() == OpcodeText
}

func (fr *Frame) IsBinary() bool {
	return fr.Opcode() == OpcodeBinary
}

func (fr *Frame) IsClose() bool {
	return fr.Opcode() == OpcodeClose
}

func (fr *Frame) IsPing() bool {
	return fr.Opcode() == OpcodePing
}

func (fr *Frame) IsPong() bool {
	return fr.Opcode() == OpcodePong
}

func (fr *Frame) IsControl() bool {
	return fr.IsClose() || fr.IsPing() || fr.IsPong()
}

func (fr *Frame) IsMasked() bool {
	return fr.header[1]&maskBit != 0
}

func (fr *Frame) SetFin() {
	fr.header[0] |= finBit
}

func (fr *Frame) SetRSV1() {
	fr.header[0] |= rsv1Bit
}

func (fr *Frame) SetRSV2() {
	fr.header[0] |= rsv2Bit
}

func (fr *Frame) SetRSV3() {
	fr.header[0] |= rsv3Bit
}

func (fr *Frame) SetOpcode(c Opcode) {
	c &= 15
	fr.header[0] &= 15 << 4
	fr.header[0] |= uint8(c)
}

func (fr *Frame) SetContinuation() {
	fr.SetOpcode(OpcodeContinuation)
}

func (fr *Frame) SetText() {
	fr.SetOpcode(OpcodeText)
}

func (fr *Frame) SetBinary() {
	fr.SetOpcode(OpcodeBinary)
}

func (fr *Frame) SetClose() {
	fr.SetOpcode(OpcodeClose)
}

func (fr *Frame) SetPing() {
	fr.SetOpcode(OpcodePing)
}

func (fr *Frame) SetPong() {
	fr.SetOpcode(OpcodePong)
}

func (fr *Frame) SetMask(b []byte) {
	fr.header[1] |= maskBit
	copy(fr.mask, b[:4])
}

func (fr *Frame) UnsetMask() {
	fr.header[1] ^= maskBit
}

func (fr *Frame) SetPayload(b []byte) {
	n := 0
	if fr.IsClose() {
		n = 2
		if cap(fr.payload) < 2 {
			fr.payload = append(fr.payload, make([]byte, 2)...)
		}
	}
	fr.payload = append(fr.payload[:n], b...)
}

func (fr *Frame) Len() uint64 {
	length := uint64(fr.header[1] & 127)
	switch length {
	case 126:
		length = uint64(binary.BigEndian.Uint16(fr.header[2:]))
	case 127:
		length = uint64(binary.BigEndian.Uint64(fr.header[2:]))
	}
	return length
}

func (fr *Frame) MaskKey() []byte {
	return fr.mask
}

func (fr *Frame) Payload() []byte {
	return fr.payload // TODO might be different for close frames
}

func (fr *Frame) Mask() error {
	panic("implement me")
}

func (fr *Frame) Unmask() error {
	panic("implement me")
}

// String returns a representation of Frame in a human-readable string format.
func (fr *Frame) String() string {
	return fmt.Sprintf(`
FIN: %v
RSV1: %v
RSV2: %v
RSV3: %v
--------
OPCODE: %d
--------
MASK: %v
--------
LENGTH: %d
--------
MASK-KEY: %v`,
		fr.IsFin(), fr.IsRSV1(), fr.IsRSV2(), fr.IsRSV3(),
		fr.Opcode(), fr.IsMasked(), fr.Len(), fr.MaskKey(),
	)
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
