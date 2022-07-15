package websocket

import (
	"testing"

	"github.com/talostrading/sonic"
)

func TestFrameCodecDecode(t *testing.T) {
	src := sonic.NewBytesBuffer()

	frame := NewFrame()
	state := NewFrameCodecState(frame)
	codec := NewFrameCodec(state)

	frame, err := codec.Decode(src)
	if err != nil {
		t.Fatal(err)
	}
}
