package websocket

import "github.com/talostrading/sonic"

var _ sonic.Codec[*Frame] = &FrameCodec{}

type FrameCodec struct {
	dfr *Frame
}

func NewFrameCodec() *FrameCodec {
	return &FrameCodec{}
}

func (c *FrameCodec) Decode(src *sonic.BytesBuffer) (*Frame, error) {
	// here the frame payload can be set to point to the underlying src bytes slice
	//c.fr.Reset()
	// decode...
	c.dfr.Reset()

	c.dfr.ReadFrom(src)

	return c.dfr, nil
}

func (c *FrameCodec) Encode(fr *Frame, dst *sonic.BytesBuffer) error {
	return nil
}
