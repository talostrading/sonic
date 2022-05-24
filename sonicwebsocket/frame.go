package sonicwebsocket

import (
	"encoding/binary"
	"fmt"
	"io"
)

var (
	_ io.ReadWriter = &Frame{}
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
		payload: make([]byte, 1<<20), // TODO maybe const payload size or set it beforehand?
	}
}

func (fr *Frame) Read(b []byte) (n int, err error) {
	panic("implement me")
}

func (fr *Frame) Write(b []byte) (n int, err error) {
	panic("implement me")
}

func (fr *Frame) mustRead() (n int) {
	switch fr.header[1] & 127 {
	case 127:
		n = 8
	case 126:
		n = 2
	}
	return
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
	return fmt.Sprintf(`FIN: %v
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
