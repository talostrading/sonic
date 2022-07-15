package websocket

import (
	"bytes"
	"testing"

	"github.com/talostrading/sonic"
)

func TestDecodeShort(t *testing.T) {
	src := sonic.NewBytesBuffer()
	src.Write([]byte{0x81, 1}) // fin=1 opcode=1 (text) payload_len=1

	codec := NewFrameCodec(NewFrame(), src, nil)

	f, err := codec.Decode(src)
	if err != nil {
		t.Fatal(err)
	}
	if f != nil {
		t.Fatal(err)
	}

	if codec.decodeReset {
		t.Fatal("should not reset decoder state")
	}
	if codec.decodeBytes != 0 {
		t.Fatal("should have not decoded any bytes")
	}
}

func TestDecodeExactlyOne(t *testing.T) {
	src := sonic.NewBytesBuffer()
	src.Write([]byte{0x81, 1, 0xFF}) // fin=1 opcode=1 (text) payload_len=1

	codec := NewFrameCodec(NewFrame(), src, nil)

	f, err := codec.Decode(src)
	if err != nil {
		t.Fatal(err)
	}
	if f == nil {
		t.Fatal(err)
	}

	if !(f.IsFin() && f.IsText() && bytes.Equal(f.Payload(), []byte{0xFF})) {
		t.Fatal("corrupt frame")
	}

	if !codec.decodeReset {
		t.Fatal("should reset decoder state")
	}
	if codec.decodeBytes != 3 {
		t.Fatal("should have decoded 3 bytes")
	}
}

func TestDecodeMoreThanOne(t *testing.T) {
	src := sonic.NewBytesBuffer()
	src.Write([]byte{0x81, 1, 0xFF, 0xFF, 0xFF, 0xFF}) // fin=1 opcode=1 (text) payload_len=1

	codec := NewFrameCodec(NewFrame(), src, nil)

	f, err := codec.Decode(src)
	if err != nil {
		t.Fatal(err)
	}
	if f == nil {
		t.Fatal(err)
	}

	if !(f.IsFin() && f.IsText() && bytes.Equal(f.Payload(), []byte{0xFF})) {
		t.Fatal("corrupt frame")
	}

	if !codec.decodeReset {
		t.Fatal("should reset decoder state")
	}

	if codec.decodeBytes != 3 {
		t.Fatal("should have decoded 3 bytes")
	}
}
