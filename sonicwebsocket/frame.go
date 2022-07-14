package sonicwebsocket

import (
	"encoding/binary"
	"fmt"
	"io"
	"sync"

	"github.com/talostrading/sonic/util"
)

var zeroBytes = []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}

var (
	_ io.ReaderFrom = &Frame{}
	_ io.WriterTo   = &Frame{}
)

type FrameHeader struct {
	header []byte
	mask   []byte
}

func NewFrameHeader() *FrameHeader {
	return &FrameHeader{
		header: make([]byte, FrameHeaderSize),
		mask:   make([]byte, FrameMaskSize),
	}
}

func (h *FrameHeader) CopyTo(dst *FrameHeader) {
	dst.header = dst.header[:cap(dst.header)]
	n := copy(dst.header, h.header)
	dst.header = dst.header[:n]

	dst.mask = dst.mask[:cap(dst.mask)]
	n = copy(dst.mask, h.mask)
	dst.mask = dst.mask[:n]
}

func (h *FrameHeader) CopyToFrame(dst *Frame) {
	h.CopyTo(dst.FrameHeader)
}

func (h *FrameHeader) Read(b []byte, r io.Reader) (int64, error) {
	var nt int64 = 0

	n, err := io.ReadFull(r, h.header[:2])
	nt += int64(n)
	if err == nil {
		more := h.readMore()
		if more > 0 {
			n, err = io.ReadFull(r, h.header[2:more+2])
			nt += int64(n)

			if err == nil {
				if h.PayloadLen() > MaxFramePayloadSize {
					err = ErrPayloadTooBig
				}
			}
		}

		if err == nil && h.IsMasked() {
			n, err = io.ReadFull(r, h.mask[:4])
			nt += int64(n)
		}

		if err == nil {
			if pn := f.PayloadLen(); pn > 0 {
				if pn > MaxFramePayloadSize {
					return nt, ErrPayloadTooBig
				} else {
					f.payload = util.ExtendBytes(f.payload, pn)

					var n int
					n, err = io.ReadFull(r, f.payload[:pn])
					nt += int64(n)
				}
			} else if pn == 0 {
				f.payload = f.payload[:0]
			}
		}

	}

	return nt, err
}

func (h *FrameHeader) Write(b []byte, w io.Writer) (int, error) {
	nt := 0

	n, err := w.Write(h.header[:2+h.setPayloadLen(b)])
	nt += n

	if err == nil {
		if h.IsMasked() {
			n, err = w.Write(h.mask[:])
			nt += n
		}

		if err == nil && len(b) > 0 {
			n, err = w.Write(b)
			nt += n
		}
	}

	return nt, err
}

func (h *FrameHeader) setPayloadLen(b []byte) (bytes int) {
	n := len(b)

	switch {
	case n > 65535: // more than two bytes needed for extra length
		bytes = 8
		h.header[1] |= uint8(127)
		binary.BigEndian.PutUint64(h.header[2:], uint64(n))
	case n > 125:
		bytes = 2
		h.header[1] |= uint8(126)
		binary.BigEndian.PutUint16(h.header[2:], uint16(n))
		return 2
	default:
		bytes = 0
		h.header[1] |= uint8(n)
	}
	return
}

func (h *FrameHeader) readMore() (n int) {
	switch h.header[1] & 127 {
	case 127:
		n = 8
	case 126:
		n = 2
	}
	return
}

func (h *FrameHeader) Reset() {
	copy(h.header[:], zeroBytes)
	copy(h.mask[:], zeroBytes)
}

func (h *FrameHeader) IsFin() bool {
	return h.header[0]&finBit != 0
}

func (h *FrameHeader) IsRSV1() bool {
	return h.header[0]&rsv1Bit != 0
}

func (h *FrameHeader) IsRSV2() bool {
	return h.header[0]&rsv2Bit != 0
}

func (h *FrameHeader) IsRSV3() bool {
	return h.header[0]&rsv3Bit != 0
}

func (h *FrameHeader) Opcode() Opcode {
	return Opcode(h.header[0] & 15)
}

func (h *FrameHeader) IsContinuation() bool {
	return h.Opcode() == OpcodeContinuation
}

func (h *FrameHeader) IsText() bool {
	return h.Opcode() == OpcodeText
}

func (h *FrameHeader) IsBinary() bool {
	return h.Opcode() == OpcodeBinary
}

func (h *FrameHeader) IsClose() bool {
	return h.Opcode() == OpcodeClose
}

func (h *FrameHeader) IsPing() bool {
	return h.Opcode() == OpcodePing
}

func (h *FrameHeader) IsPong() bool {
	return h.Opcode() == OpcodePong
}

func (h *FrameHeader) IsControl() bool {
	return h.IsClose() || h.IsPing() || h.IsPong()
}

func (h *FrameHeader) IsMasked() bool {
	return h.header[1]&maskBit != 0
}

func (h *FrameHeader) SetFin() {
	h.header[0] |= finBit
}

func (h *FrameHeader) SetRSV1() {
	h.header[0] |= rsv1Bit
}

func (h *FrameHeader) SetRSV2() {
	h.header[0] |= rsv2Bit
}

func (h *FrameHeader) SetRSV3() {
	h.header[0] |= rsv3Bit
}

func (h *FrameHeader) SetOpcode(c Opcode) {
	c &= 15
	h.header[0] &= 15 << 4
	h.header[0] |= uint8(c)
}

func (h *FrameHeader) SetContinuation() {
	h.SetOpcode(OpcodeContinuation)
}

func (h *FrameHeader) SetText() {
	h.SetOpcode(OpcodeText)
}

func (h *FrameHeader) SetBinary() {
	h.SetOpcode(OpcodeBinary)
}

func (h *FrameHeader) SetClose() {
	h.SetOpcode(OpcodeClose)
}

func (h *FrameHeader) SetPing() {
	h.SetOpcode(OpcodePing)
}

func (h *FrameHeader) SetPong() {
	h.SetOpcode(OpcodePong)
}

func (h *FrameHeader) PayloadLen() int {
	length := uint64(h.header[1] & 127)
	switch length {
	case 126:
		length = uint64(binary.BigEndian.Uint16(h.header[2:]))
	case 127:
		length = binary.BigEndian.Uint64(h.header[2:])
	}
	return int(length)
}

func (h *FrameHeader) MaskKey() []byte {
	return h.mask[:]
}

func (h *FrameHeader) Mask(b []byte) {
	h.header[1] |= maskBit
	genMask(h.mask[:])
	if len(b) > 0 {
		mask(h.mask[:], b)
	}
}

func (h *FrameHeader) Unmask(b []byte) {
	h.header[1] ^= maskBit
	if len(b) > 0 {
		key := h.MaskKey()
		mask(key, b)
	}
}

func (h *FrameHeader) String() string {
	return fmt.Sprintf(`
FIN: %v
--------
OPCODE: %d
--------
MASK: %v
--------
LENGTH: %d
--------
MASK-KEY: %v
`,
		h.IsFin(), h.Opcode(), h.IsMasked(),
		h.PayloadLen(), h.MaskKey(),
	)
}

type Frame struct {
	*FrameHeader
	payload []byte
}

func NewFrame() *Frame {
	f := &Frame{}
	f.FrameHeader = NewFrameHeader()
	f.payload = make([]byte, DefaultPayloadSize)
	return f
}

func (fr *Frame) CopyTo(dst *Frame) {
	fr.FrameHeader.CopyTo(dst.FrameHeader)

	dst.payload = dst.payload[:cap(dst.payload)]
	n := copy(dst.payload, fr.payload)
	dst.payload = dst.payload[:n]
}

func (f *Frame) ReadFrom(r io.Reader) (int64, error) {
	var nt int64 = 0

	n, err := f.FrameHeader.ReadFrom(r)
	nt += int64(n)

	if err == nil {
	}
	return int64(n), err
}

func (f *Frame) WriteTo(w io.Writer) (int64, error) {
	n, err := f.Write(f.payload, w)
	return int64(n), err
}

func (f *Frame) Reset() {
	f.FrameHeader.Reset()
	f.payload = f.payload[:0]
}

func (f *Frame) SetPayload(b []byte) {
	f.payload = append(f.payload[:0], b...)
}

func (f *Frame) Payload() []byte {
	return f.payload
}

func (f *Frame) Mask(_ []byte) {
	f.FrameHeader.Mask(f.payload)
}

func (f *Frame) Unmask(_ []byte) {
	f.FrameHeader.Unmask(f.payload)
}

func (fr *Frame) String() string {
	return fmt.Sprintf(`
FIN: %v
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

		fr.IsFin(), fr.Opcode(), fr.IsMasked(),
		fr.PayloadLen(), fr.MaskKey(), fr.Payload(),
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

var frameHeaderPool = sync.Pool{
	New: func() interface{} {
		return NewFrameHeader()
	},
}

func AcquireFrameHeader() *FrameHeader {
	return frameHeaderPool.Get().(*FrameHeader)
}

func ReleaseFrameHeader(h *FrameHeader) {
	h.Reset()
	frameHeaderPool.Put(h)
}
