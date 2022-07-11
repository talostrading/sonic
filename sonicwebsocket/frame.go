package sonicwebsocket

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"sync"

	"github.com/talostrading/sonic/util"
)

var zeroBytes = []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0}

var (
	_ io.ReaderFrom = &Frame{}
	_ io.WriterTo   = &Frame{}
)

type Frame struct {
	header  []byte
	mask    []byte
	payload []byte
}

func newFrame() *Frame {
	return &Frame{
		header:  make([]byte, frameHeaderSize),
		mask:    make([]byte, frameMaskSize),
		payload: make([]byte, DefaultPayloadSize),
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

			if err == nil {
				if fr.Len() > MaxMessageSize {
					return int64(n), ErrPayloadTooBig
				}
			}
		}

		if err == nil && fr.IsMasked() {
			n, err = io.ReadFull(r, fr.mask[:4])
			if err == io.ErrUnexpectedEOF {
				err = ErrReadingMask
			}
		}

		if err == nil {
			if payloadLen := int(fr.Len()); payloadLen > 0 {
				fr.payload = util.ExtendByteSlice(fr.payload, payloadLen)
				n, err = io.ReadFull(r, fr.payload[:payloadLen])
			} else if payloadLen == 0 {
				fr.payload = fr.payload[:0]
			} else if payloadLen < 0 {
				panic("invalid uint64 to int conversion")
			}
		}
	}

	return int64(n), err
}

func (fr *Frame) CopyTo(dst *Frame) {
	dst.header = dst.header[:cap(dst.header)]
	n := copy(dst.header, fr.header)
	dst.header = dst.header[:n]

	dst.mask = dst.mask[:cap(dst.mask)]
	n = copy(dst.mask, fr.mask)
	dst.mask = dst.mask[:n]

	dst.payload = dst.payload[:cap(dst.payload)]
	n = copy(dst.payload, fr.payload)
	dst.payload = dst.payload[:n]
}

func (fr *Frame) WriteToBuffer(b []byte) (n int, err error) {
	b = b[:cap(b)]

	buf := bytes.NewBuffer(b)
	writer := bufio.NewWriter(buf)

	nn, err := fr.WriteTo(writer)
	return int(nn), err
}

func (fr *Frame) WriteTo(w io.Writer) (n int64, err error) {
	var nn int

	nn, err = w.Write(fr.header[:2+fr.setPayloadLen()])
	n += int64(nn)

	if err == nil {
		if fr.IsMasked() {
			nn, err = w.Write(fr.mask[:])
			n += int64(nn)
		}

		if err == nil && len(fr.payload) > 0 {
			nn, err = w.Write(fr.payload)
			n += int64(nn)
		}
	}

	return
}

func (fr *Frame) setPayloadLen() (bytes int) {
	n := len(fr.payload)

	switch {
	case n > 65535: // more than two bytes needed for extra length
		bytes = 8
		fr.setLength(127)
		binary.BigEndian.PutUint64(fr.header[2:], uint64(n))
	case n > 125:
		bytes = 2
		fr.setLength(126)
		binary.BigEndian.PutUint16(fr.header[2:], uint16(n))
		return 2
	default:
		bytes = 0
		fr.setLength(n)
	}
	return
}

func (fr *Frame) setLength(n int) {
	fr.header[1] |= uint8(n)
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
	copy(fr.header[:], zeroBytes)
	copy(fr.mask[:], zeroBytes)
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
	copy(fr.mask[:], b[:4])
}

func (fr *Frame) UnsetMask() {
	fr.header[1] ^= maskBit
}

func (fr *Frame) SetPayload(b []byte) {
	fr.payload = append(fr.payload[:0], b...)
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
	return fr.mask[:]
}

func (fr *Frame) Payload() []byte {
	return fr.payload // TODO might be different for close frames
}

func (fr *Frame) Mask() {
	fr.header[1] |= maskBit
	genMask(fr.mask[:])
	if len(fr.payload) > 0 {
		mask(fr.mask[:], fr.payload)
	}
}

func (fr *Frame) Unmask() {
	if len(fr.payload) > 0 {
		key := fr.MaskKey()
		mask(key, fr.payload)
	}
	fr.UnsetMask()
}

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
MASK-KEY: %v
--------
PAYLOAD: %v`,

		fr.IsFin(), fr.IsRSV1(), fr.IsRSV2(), fr.IsRSV3(),
		fr.Opcode(), fr.IsMasked(), fr.Len(), fr.MaskKey(),
		fr.Payload(),
	)
}

var framePool = sync.Pool{
	New: func() interface{} {
		return newFrame()
	},
}

func AcquireFrame() *Frame {
	return framePool.Get().(*Frame)
}

func ReleaseFrame(f *Frame) {
	f.Reset()
	framePool.Put(f)
}
