package websocket

import (
	"github.com/talostrading/sonic"
)

var _ sonic.Codec[*Frame] = &FrameCodec{}

// FrameCodecState is the state machine of the frame codec.
//
// The state machine reset on every successful decode of a frame.
type FrameCodecState struct {
	// The frame in which the raw bytes are decoded. Not owned by
	// the state machine.
	f *Frame

	// The number of bytes read in the current state machine run.
	readBytes int
}

func NewFrameCodecState(f *Frame) *FrameCodecState {
	s := &FrameCodecState{
		f: f,
	}
	return s
}

// Reset prepares the state machine for a new frame decode run.
func (s *FrameCodecState) Reset(src *sonic.BytesBuffer) {
	src.Consume(s.readBytes)

	s.f.Reset()
	s.readBytes = 0
}

// FrameCodec is a stateful streaming parser handling the encoding
// and decoding of WebSocket frames.
type FrameCodec struct {
	state *FrameCodecState
}

func NewFrameCodec(state *FrameCodecState) *FrameCodec {
	return &FrameCodec{
		state: state,
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
	need := 2
	if src.ReadLen() < need {
		if more := src.ReadLen() - need; src.WriteLen() >= more {
			src.Commit(more)
		} else {
			return nil, nil
		}
	}

	c.state.f.header = src.Data()[:need]
	if extra := c.state.f.ExtraHeaderLen(); extra > 0 {
		need += extra
		if src.ReadLen() < need {
			if more := src.ReadLen() - need; src.WriteLen() >= more {
				src.Commit(more)
			} else {
				return nil, nil
			}
		}
		c.state.f.header = src.Data()[:need]

		if c.state.f.PayloadLen() > MaxPayloadLen {
			return nil, ErrPayloadTooBig
		}

	}

	if c.state.f.IsMasked() {
		need += 4
		if src.ReadLen() < need {
			if more := src.ReadLen() - need; src.WriteLen() >= more {
				src.Commit(more)
			} else {
				return nil, nil
			}
		}
		c.state.f.mask = src.Data()[need-4 : need]
	}

	pn := c.state.f.PayloadLen()
	need += pn
	if src.ReadLen() < need {
		if more := src.ReadLen() - need; src.WriteLen() >= more {
			src.Commit(more)
		}
		c.state.f.payload = src.Data()[need-pn : need]

		// We parsed a frame so we can reset the state on the next decode call.
		c.state.readBytes = need

		return c.state.f, nil
	}

	return nil, nil
}

// Encode encodes the frame and place the raw bytes into `dst`.
func (c *FrameCodec) Encode(fr *Frame, dst *sonic.BytesBuffer) error {
	return nil
}
