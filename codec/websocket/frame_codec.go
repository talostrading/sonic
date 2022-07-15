package websocket

import (
	"github.com/talostrading/sonic"
)

var _ sonic.Codec[*Frame] = &FrameCodec{}

// FrameCodec is a stateful streaming parser handling the encoding
// and decoding of WebSocket frames.
type FrameCodec struct {
	src *sonic.BytesBuffer // buffer we decode from
	dst *sonic.BytesBuffer // buffer we encode to

	decodeFrame *Frame // frame we decode into
	decodeBytes int    // the number of bytes of the last successfully decoded frame
	decodeReset bool   // true if we must reset the state on the next decode
}

func NewFrameCodec(frame *Frame, src, dst *sonic.BytesBuffer) *FrameCodec {
	return &FrameCodec{
		decodeFrame: frame,
		src:         src,
		dst:         dst,
	}
}

func (c *FrameCodec) tryDecodeReset() {
	if c.decodeReset {
		c.decodeReset = false
		c.src.Consume(c.decodeBytes)
		c.decodeBytes = 0
	}
}

// Decode decodes the raw bytes from `src` into a frame.
//
// Three things can happen while decoding a raw stream of bytes into a frame:
// 1. There are not enough bytes to construct a frame with.
//
//	In this case, a nil frame and error are returned. The caller
//	should perform another read into `src` later.
//
// 2. `src` contains the bytes of one frame.
//
//	In this case we try to decode the frame. An appropriate error is returned
//	if the frame is corrupt.
//
// 3. `src` contains the bytes of more than one frame.
//
//	In this case we try to decode the first frame. The rest of the bytes stay
//	in `src`. An appropriate error is returned if the frame is corrupt.
func (c *FrameCodec) Decode(src *sonic.BytesBuffer) (*Frame, error) {
	c.tryDecodeReset()

	// read fixed size header
	n := 2
	err := src.PrepareRead(n)
	if err != nil {
		return nil, nil
	}
	c.decodeFrame.header = src.Data()[:n]

	// read extra header length
	n += c.decodeFrame.ExtraHeaderLen()
	if err := src.PrepareRead(n); err != nil {
		return nil, nil
	}
	c.decodeFrame.header = src.Data()[:n]

	// read mask if any
	if c.decodeFrame.IsMasked() {
		n += 4
		if err := src.PrepareRead(n); err != nil {
			return nil, nil
		}
		c.decodeFrame.mask = src.Data()[n-4 : n]
	}

	// check payload length
	npayload := c.decodeFrame.PayloadLen()
	if npayload > MaxPayloadLen {
		return nil, ErrPayloadTooBig
	}

	// prepare to read the payload
	n += npayload
	if err := src.PrepareRead(n); err != nil {
		return nil, nil
	}

	// at this point, we have a full frame in src
	c.decodeFrame.payload = src.Data()[n-npayload : n]
	c.decodeBytes = n
	c.decodeReset = true

	return c.decodeFrame, nil
}

// Encode encodes the frame and place the raw bytes into `dst`.
func (c *FrameCodec) Encode(fr *Frame, dst *sonic.BytesBuffer) error {
	return nil
}
