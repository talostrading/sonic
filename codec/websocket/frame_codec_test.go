package websocket

import (
	"bytes"
	"errors"
	"testing"

	"github.com/talostrading/sonic"
	"github.com/talostrading/sonic/sonicerrors"
)

func TestDecodeShortFrame(t *testing.T) {
	src := sonic.NewByteBuffer()
	src.Write([]byte{0x81, 1}) // fin=1 opcode=1 (text) payload_len=1

	codec := NewFrameCodec(src, nil)

	f, err := codec.Decode(src)
	if !errors.Is(err, sonicerrors.ErrNeedMore) {
		t.Fatal("should have gotten ErrNeedMore")
	}
	if f != nil {
		t.Fatal("should not have gotten a frame")
	}
	if codec.decodeReset {
		t.Fatal("should not reset decoder state")
	}
	if len(codec.decodeFrame) != 0 {
		t.Fatal("should have not decoded any bytes")
	}
}

func TestDecodeExactlyOneFrame(t *testing.T) {
	src := sonic.NewByteBuffer()
	src.Write([]byte{0x81, 1, 0xFF}) // fin=1 opcode=1 (text) payload_len=1

	codec := NewFrameCodec(src, nil)

	f, err := codec.Decode(src)
	if err != nil {
		t.Fatal(err)
	}
	if f == nil {
		t.Fatal("should have gotten a frame")
	}

	if !(f.IsFIN() && f.Opcode().IsText() && bytes.Equal(f.Payload(), []byte{0xFF})) {
		t.Fatal("corrupt frame")
	}

	if !codec.decodeReset {
		t.Fatal("should reset decoder state")
	}
	if len(codec.decodeFrame) != 3 {
		t.Fatal("should have decoded 3 bytes")
	}
}

func TestDecodeOneAndShortFrame(t *testing.T) {
	src := sonic.NewByteBuffer()

	// fin=1 opcode=1 (text) payload_len=1
	src.Write([]byte{0x81, 1, 0xFF, 0xFF, 0xFF, 0xFF})

	codec := NewFrameCodec(src, nil)

	f, err := codec.Decode(src)
	if err != nil {
		t.Fatal(err)
	}
	if f == nil {
		t.Fatal("should have gotten a frame")
	}

	if !(f.IsFIN() && f.Opcode().IsText() && bytes.Equal(f.Payload(), []byte{0xFF})) {
		t.Fatal("corrupt frame")
	}

	if !codec.decodeReset {
		t.Fatal("should reset decoder state")
	}

	if len(codec.decodeFrame) != 3 {
		t.Fatal("should have decoded 3 bytes")
	}
}

func TestDecodeTwoFrames(t *testing.T) {
	src := sonic.NewByteBuffer()
	src.Write([]byte{
		0x81, 1, 0xFF, // first complete frame
		0x81, 5, 0x01, 0x02, 0x03, 0x04, 0x05, // second complete frame
		0x81, 10}) // third short frame

	codec := NewFrameCodec(src, nil)

	if src.WriteLen() != 12 {
		t.Fatal("should have 12 bytes in the write area")
	}

	// first frame
	f, err := codec.Decode(src)
	if err != nil {
		t.Fatal(err)
	}
	if f == nil {
		t.Fatal("should have gotten a frame")
	}
	if !(f.IsFIN() && f.Opcode().IsText() && bytes.Equal(f.Payload(), []byte{0xFF})) {
		t.Fatal("corrupt frame")
	}
	if !codec.decodeReset {
		t.Fatal("should have reset decoder state")
	}
	if len(codec.decodeFrame) != 3 {
		t.Fatal("should have decoded 3 bytes")
	}
	if src.ReadLen() != 3 {
		t.Fatal("should have 3 bytes in read area")
	}

	// second frame
	f, err = codec.Decode(src)
	if err != nil {
		t.Fatal(err)
	}
	if f == nil {
		t.Fatal("should have gotten a frame")
	}
	if !(f.IsFIN() &&
		f.Opcode().IsText() &&
		bytes.Equal(f.Payload(), []byte{0x01, 0x02, 0x03, 0x04, 0x05})) {
		t.Fatal("corrupt frame")
	}
	if !codec.decodeReset {
		t.Fatal("should have reset decoder state")
	}
	if len(codec.decodeFrame) != 7 {
		t.Fatal("should have decoded 3 bytes")
	}
	if src.ReadLen() != 7 {
		t.Fatal("should have 7 bytes in read area")
	}
	if src.WriteLen() != 2 {
		t.Fatal("should have 2 bytes in write area")
	}

	// third try on short frame
	f, err = codec.Decode(src)
	if !errors.Is(err, sonicerrors.ErrNeedMore) {
		t.Fatal(err)
	}
	if f != nil {
		t.Fatal("should not have gotten a frame")
	}
	if src.ReadLen() != 2 {
		t.Fatal("should have 2 bytes in the read area")
	}
	if src.WriteLen() != 0 {
		t.Fatal("should have 0 bytes in the write area")
	}
}
