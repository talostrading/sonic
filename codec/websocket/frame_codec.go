package websocket

import (
	"errors"

	"github.com/talostrading/sonic"
)

var _ sonic.Codec[Frame, Frame] = &FrameCodec{}

var (
	ErrPartialPayload = errors.New("partial payload")
)

// FrameCodec is a stateful streaming parser handling the encoding and decoding of WebSocket `Frame`s.
type FrameCodec struct {
	src *sonic.ByteBuffer // buffer we decode from
	dst *sonic.ByteBuffer // buffer we encode to

	decodeFrame Frame // frame we decode into
	decodeReset bool  // true if we must reset the state on the next decode
}

func NewFrameCodec(src, dst *sonic.ByteBuffer) *FrameCodec {
	return &FrameCodec{
		decodeFrame: NewFrame(),
		src:         src,
		dst:         dst,
	}
}

func (c *FrameCodec) resetDecode() {
	if c.decodeReset {
		c.decodeReset = false
		c.src.Consume(len(c.decodeFrame))
		c.decodeFrame = nil
	}
}

// Decode decodes the raw bytes from `src` into a `Frame`.
//
// Two things can happen while decoding a raw stream of bytes into a frame:
//
// 1. There are not enough bytes to construct a frame with: in this case, a `nil` `Frame` and `ErrNeedMore` are
// returned. The caller should perform another read into `src` later.
//
// 2. `src` contains at least the bytes of one `Frame`: we decode the next `Frame` and leave the remainder bytes
// composing a partial `Frame` or a set of `Frame`s in the `src` buffer.
func (c *FrameCodec) Decode(src *sonic.ByteBuffer) (Frame, error) {
	c.resetDecode()

	// read the mandatory header
	readSoFar := frameHeaderLength
	if err := src.PrepareRead(readSoFar); err != nil {
		c.decodeFrame = nil
		return nil, err
	}
	c.decodeFrame = src.Data()[:readSoFar]

	// read the extended payload length (0, 2 or 8 bytes) and check if within bounds
	readSoFar += c.decodeFrame.ExtendedPayloadLengthBytes()
	if err := src.PrepareRead(readSoFar); err != nil {
		c.decodeFrame = nil
		return nil, err
	}
	c.decodeFrame = src.Data()[:readSoFar]

	payloadLength := c.decodeFrame.PayloadLength()
	if payloadLength > MaxMessageSize {
		c.decodeFrame = nil
		return nil, ErrPayloadOverMaxSize
	}

	// read mask if any
	if c.decodeFrame.IsMasked() {
		readSoFar += frameMaskLength
		if err := src.PrepareRead(readSoFar); err != nil {
			c.decodeFrame = nil
			return nil, err
		}
		c.decodeFrame = src.Data()[:readSoFar]
	}

	// read the payload; if that succeeds, we have a full frame in `src` - the decoding was successful and we can return
	// the frame
	readSoFar += payloadLength
	if err := src.PrepareRead(readSoFar); err != nil {
		src.Reserve(payloadLength) // payload is incomplete; reserve enough space for the remainder to fit in the buffer
		c.decodeFrame = nil
		return nil, err
	}
	c.decodeFrame = src.Data()[:readSoFar]
	c.decodeReset = true

	return c.decodeFrame, nil
}

// Encode encodes the `Frame` into `dst`.
func (c *FrameCodec) Encode(frame Frame, dst *sonic.ByteBuffer) error {
	// ensure the destination buffer can hold the serialized frame // TODO this can be improved
	dst.Reserve(frame.PayloadLength() + MaxFrameHeaderLengthInBytes)

	n, err := frame.WriteTo(dst)
	dst.Commit(int(n))
	if err != nil {
		dst.Consume(int(n))
	}
	return err
}
