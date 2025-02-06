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

	// TODO should be renamed to maxFrameSize? or cummulate the size of all fragments from a message
	maxMessageSize int

	decodeFrame Frame // frame we decode into
	decodeReset bool  // true if we must reset the state on the next decode

	messagePayloads [][]byte 	 // references to reserved frame payloads
	messageSize int 					 // total size of reserved frame payloads
}

func NewFrameCodec(src, dst *sonic.ByteBuffer, maxMessageSize int) *FrameCodec {
	return &FrameCodec{
		decodeFrame:    NewFrame(),
		src:            src,
		dst:            dst,
		maxMessageSize: maxMessageSize,
		messagePayloads: make([][]byte, 0, 32), // preallocate 32 frame payload references
		messageSize: 0,
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
	if payloadLength > c.maxMessageSize {
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
	// TODO this can be improved: we can serialize directly in the buffer with zero-copy semantics

	// ensure the destination buffer can hold the serialized frame
	dst.Reserve(frame.PayloadLength() + frameMaxHeaderLength)

	n, err := frame.WriteTo(dst)
	dst.Commit(int(n))
	if err != nil {
		dst.Consume(int(n))
	}
	return err
}

// ReserveFrame saves a frame so that we can refer to it after we decode the next frame
func (c *FrameCodec) ReserveFrame() {
	if c.decodeFrame == nil {
		panic("No decoded frame to reserve")
	}

	// Move memory backing decodeFrame from read to save area
	frameLen := len(c.decodeFrame)
	c.src.Save(frameLen)

	// Since we're holding on to the area of the buffer holding this frame
	// by moving it from the read to save area, we shouldn't also 
	// try to discard this area in resetDecode()
	// 
	// (you could think of this almost like preventing a double-free error)
	c.decodeReset = false

	framePayload := c.decodeFrame.Payload()

	// Store reference to frame payload
	c.messagePayloads = append(c.messagePayloads, framePayload)
	c.messageSize += len(framePayload)
}

// ReleaseFrames releases our reserved frames
func (c *FrameCodec) ReleaseFrames() {
	c.src.DiscardAll()
	c.messagePayloads = c.messagePayloads[:0]
	c.messageSize = 0
}

// ReservedFramePayloads returns references to the payloads of our reserved frames
func (c *FrameCodec) ReservedFramePayloads() [][]byte {
	return c.messagePayloads;
}