package sonicwebsocket

import (
	"encoding/binary"
	"fmt"
	"io"
	"sync"

	"github.com/talostrading/sonic/util"
)

var zeroBytes = []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0}

var (
	_ io.ReaderFrom = &frame{}
	_ io.WriterTo   = &frame{}
)

type frame struct {
	header  [frameHeaderSize]byte
	mask    [frameMaskSize]byte
	payload []byte
}

func newFrame() *frame {
	return &frame{
		payload: make([]byte, DefaultPayloadSize),
	}
}

func (fr *frame) ReadFrom(r io.Reader) (int64, error) {
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

func (fr *frame) WriteTo(w io.Writer) (n int64, err error) {
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

func (fr *frame) setPayloadLen() (bytes int) {
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

func (fr *frame) setLength(n int) {
	fr.header[1] |= uint8(n)
}

func (fr *frame) readMore() (n int) {
	switch fr.header[1] & 127 {
	case 127:
		n = 8
	case 126:
		n = 2
	}
	return
}

func (fr *frame) Reset() {
	copy(fr.header[:], zeroBytes)
	copy(fr.mask[:], zeroBytes)
}

func (fr *frame) IsFin() bool {
	return fr.header[0]&finBit != 0
}

func (fr *frame) IsRSV1() bool {
	return fr.header[0]&rsv1Bit != 0
}

func (fr *frame) IsRSV2() bool {
	return fr.header[0]&rsv2Bit != 0
}

func (fr *frame) IsRSV3() bool {
	return fr.header[0]&rsv3Bit != 0
}

func (fr *frame) Opcode() Opcode {
	return Opcode(fr.header[0] & 15)
}

func (fr *frame) IsContinuation() bool {
	return fr.Opcode() == OpcodeContinuation
}

func (fr *frame) IsText() bool {
	return fr.Opcode() == OpcodeText
}

func (fr *frame) IsBinary() bool {
	return fr.Opcode() == OpcodeBinary
}

func (fr *frame) IsClose() bool {
	return fr.Opcode() == OpcodeClose
}

func (fr *frame) IsPing() bool {
	return fr.Opcode() == OpcodePing
}

func (fr *frame) IsPong() bool {
	return fr.Opcode() == OpcodePong
}

func (fr *frame) IsControl() bool {
	return fr.IsClose() || fr.IsPing() || fr.IsPong()
}

func (fr *frame) IsMasked() bool {
	return fr.header[1]&maskBit != 0
}

func (fr *frame) SetFin() {
	fr.header[0] |= finBit
}

func (fr *frame) SetRSV1() {
	fr.header[0] |= rsv1Bit
}

func (fr *frame) SetRSV2() {
	fr.header[0] |= rsv2Bit
}

func (fr *frame) SetRSV3() {
	fr.header[0] |= rsv3Bit
}

func (fr *frame) SetOpcode(c Opcode) {
	c &= 15
	fr.header[0] &= 15 << 4
	fr.header[0] |= uint8(c)
}

func (fr *frame) SetContinuation() {
	fr.SetOpcode(OpcodeContinuation)
}

func (fr *frame) SetText() {
	fr.SetOpcode(OpcodeText)
}

func (fr *frame) SetBinary() {
	fr.SetOpcode(OpcodeBinary)
}

func (fr *frame) SetClose() {
	fr.SetOpcode(OpcodeClose)
}

func (fr *frame) SetPing() {
	fr.SetOpcode(OpcodePing)
}

func (fr *frame) SetPong() {
	fr.SetOpcode(OpcodePong)
}

func (fr *frame) SetMask(b []byte) {
	fr.header[1] |= maskBit
	copy(fr.mask[:], b[:4])
}

func (fr *frame) UnsetMask() {
	fr.header[1] ^= maskBit
}

func (fr *frame) SetPayload(b []byte) {
	fr.payload = append(fr.payload[:0], b...)
}

func (fr *frame) Len() uint64 {
	length := uint64(fr.header[1] & 127)
	switch length {
	case 126:
		length = uint64(binary.BigEndian.Uint16(fr.header[2:]))
	case 127:
		length = uint64(binary.BigEndian.Uint64(fr.header[2:]))
	}
	return length
}

func (fr *frame) MaskKey() []byte {
	return fr.mask[:]
}

func (fr *frame) Payload() []byte {
	return fr.payload // TODO might be different for close frames
}

func (fr *frame) Mask() {
	fr.header[1] |= maskBit
	genMask(fr.mask[:])
	if len(fr.payload) > 0 {
		mask(fr.mask[:], fr.payload)
	}
}

func (fr *frame) Unmask() {
	if len(fr.payload) > 0 {
		key := fr.MaskKey()
		mask(key, fr.payload)
	}
	fr.UnsetMask()
}

func (fr *frame) String() string {
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

func AcquireFrame() *frame {
	return framePool.Get().(*frame)
}

func ReleaseFrame(f *frame) {
	f.Reset()
	framePool.Put(f)
}
