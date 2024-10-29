package websocket

import (
	"encoding/binary"
	"io"

	"github.com/talostrading/sonic/util"
)

var zeroBytes [frameMaxHeaderLength]byte

func init() {
	for i := 0; i < len(zeroBytes); i++ {
		zeroBytes[i] = 0
	}
}

type Frame []byte

var (
	_ io.ReaderFrom = &Frame{}
	_ io.WriterTo   = &Frame{}
)

// NOTE use stream.AcquireFrame() instead of NewFrame if you intend to write this frame onto a WebSocket stream.
func NewFrame() Frame {
	b := make([]byte, frameMaxHeaderLength)
	copy(b, zeroBytes[:])
	return b
}

func (f Frame) Reset() {
	copy(f, zeroBytes[:])
}

func (f Frame) ExtendedPayloadLengthBytes() int {
	if v := f[1] & bitmaskPayloadLength; v == 127 {
		return 8
	} else if v == 126 {
		return 2
	}
	return 0
}

func (f Frame) PayloadLength() int {
	if length := f[1] & bitmaskPayloadLength; length == 127 {
		return int(binary.BigEndian.Uint64(f[frameHeaderLength : frameHeaderLength+8]))
	} else if length == 126 {
		return int(binary.BigEndian.Uint16(f[frameHeaderLength : frameHeaderLength+2]))
	} else {
		return int(length)
	}
}

// An unfragmented message consists of a single frame with the FIN bit set and an opcode other than 0.
//
// A fragmented message consists of a single frame with the FIN bit clear and an opcode other than 0, followed by zero
// or more frames with the FIN bit clear and the opcode set to 0, and terminated by a single frame with the FIN bit set
// and an opcode of 0.
func (f Frame) IsFIN() bool {
	return f[0]&bitFIN != 0
}

func (f Frame) IsRSV1() bool {
	return f[0]&bitRSV1 != 0
}

func (f Frame) IsRSV2() bool {
	return f[0]&bitRSV2 != 0
}

func (f Frame) IsRSV3() bool {
	return f[0]&bitRSV3 != 0
}

func (f Frame) Opcode() Opcode {
	return Opcode(f[0] & bitmaskOpcode)
}

func (f Frame) IsMasked() bool {
	return f[1]&bitIsMasked != 0
}

func (f *Frame) SetIsMasked() *Frame {
	(*f)[1] |= bitIsMasked
	return f
}

func (f *Frame) UnsetIsMasked() *Frame {
	(*f)[1] ^= bitIsMasked
	return f
}

func (f Frame) MaskBytes() int {
	if f.IsMasked() {
		return frameMaskLength
	}
	return 0
}

func (f *Frame) SetFIN() *Frame {
	(*f)[0] |= bitFIN
	return f
}

func (f *Frame) SetRSV1() *Frame {
	(*f)[0] |= bitRSV1
	return f
}

func (f *Frame) SetRSV2() *Frame {
	(*f)[0] |= bitRSV2
	return f
}

func (f *Frame) SetRSV3() *Frame {
	(*f)[0] |= bitRSV3
	return f
}

func (f Frame) clearOpcode() {
	f[0] &= bitmaskOpcode << 4
}

func (f *Frame) SetOpcode(c Opcode) *Frame {
	c &= Opcode(bitmaskOpcode)
	f.clearOpcode()
	(*f)[0] |= byte(c)
	return f
}

func (f *Frame) SetContinuation() *Frame {
	f.SetOpcode(OpcodeContinuation)
	return f
}

func (f *Frame) SetText() *Frame {
	f.SetOpcode(OpcodeText)
	return f
}

func (f *Frame) SetBinary() *Frame {
	f.SetOpcode(OpcodeBinary)
	return f
}

func (f *Frame) SetClose() *Frame {
	f.SetOpcode(OpcodeClose)
	return f
}

func (f *Frame) SetPing() *Frame {
	f.SetOpcode(OpcodePing)
	return f
}

func (f *Frame) SetPong() *Frame {
	f.SetOpcode(OpcodePong)
	return f
}

func (f Frame) extendedPayloadLengthOffset() int {
	return frameHeaderLength
}

func (f Frame) ExtendedPayloadLength() []byte {
	if bytes := f.ExtendedPayloadLengthBytes(); bytes > 0 {
		b := f[frameHeaderLength:]
		return b[:bytes]
	}
	return nil
}

func (f Frame) Header() []byte {
	return f[:frameHeaderLength]
}

func (f *Frame) maskOffset() int {
	return frameHeaderLength + f.ExtendedPayloadLengthBytes()
}

func (f Frame) Mask() []byte {
	if f.IsMasked() {
		mask := f[f.maskOffset():]
		return mask[:frameMaskLength]
	}
	return nil
}

func (f Frame) payloadOffset() int {
	return frameHeaderLength + f.ExtendedPayloadLengthBytes() + f.MaskBytes()
}

func (f *Frame) setPayloadLength(n int) *Frame {
	(*f)[1] &= (1 << 7)

	if n > (1<<16 - 1) {
		// does not fit in 2 bytes, so take 8 as extended payload length
		(*f)[1] |= 127
		binary.BigEndian.PutUint64((*f)[2:], uint64(n))
	} else if n > 125 {
		// fits in 2 bytes as extended payload length
		(*f)[1] |= 126
		binary.BigEndian.PutUint16((*f)[2:], uint16(n))
	} else {
		// can be encoded in the 7 bits of the payload length, no extended payload length taken
		(*f)[1] |= byte(n)
	}
	return f
}

func (f *Frame) SetPayload(b []byte) *Frame {
	f.setPayloadLength(len(b)) // set the length as it's used by `payloadOffset`.

	*f = util.ExtendSlice(*f, f.payloadOffset()+len(b))
	payload := f.Payload()
	copy(payload, b)

	return f
}

func (f Frame) Payload() []byte {
	return f[f.payloadOffset():]
}

func (f *Frame) MaskPayload() {
	f.SetIsMasked()

	var (
		mask    = f.Mask()
		payload = f.Payload()
	)

	if len(payload) > 0 {
		GenMask(mask)
		Mask(mask, payload)
	}
}

func (f *Frame) UnmaskPayload() {
	if f.IsMasked() {
		var (
			mask    = f.Mask()
			payload = f.Payload()
		)
		Mask(mask, payload)
		// Does not unset the IsMasked bit in order to not mess up the offset at which the payload is found.
	}
}

func (f *Frame) fitPayload() ([]byte, error) {
	length := f.PayloadLength()
	if length <= 0 {
		return nil, nil
	}

	*f = util.ExtendSlice(*f, f.payloadOffset()+length)
	b := (*f)[f.payloadOffset():]
	return b[:length], nil
}

func (f *Frame) ReadFrom(r io.Reader) (n int64, err error) {
	var nn int

	// read the header
	nn, err = io.ReadFull(r, f.Header())
	n += int64(nn)
	if err != nil {
		return
	}

	// read the extended payload length, if any
	if b := f.ExtendedPayloadLength(); b != nil {
		nn, err = io.ReadFull(r, b)
		n += int64(nn)
		if err != nil {
			return
		}
	}

	// read the mask, if any
	if f.IsMasked() {
		nn, err = io.ReadFull(r, f.Mask())
		n += int64(nn)
		if err != nil {
			return
		}
	}

	// read the payload, if any
	b, err := f.fitPayload()
	if err != nil {
		return
	}
	if b != nil {
		nn, err = io.ReadFull(r, b)
		n += int64(nn)
		if err != nil {
			return
		}
	}

	return
}

func (f Frame) WriteTo(w io.Writer) (int64, error) {
	written := 0
	for written < len(f) {
		n, err := w.Write(f[written:])
		written += n
		if err != nil {
			return int64(n), err
		}
	}

	return int64(written), nil
}
