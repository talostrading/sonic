package frame

import (
	"encoding/binary"
	"math/rand"
	"testing"

	"github.com/csdenboer/sonic/sonicerrors"

	"github.com/csdenboer/sonic"
)

func TestFrameEncode(t *testing.T) {
	buf := sonic.NewByteBuffer()
	codec := NewCodec(buf /* does not matter since we only encode */)

	for payloadLen := 0; payloadLen < 1024*32; payloadLen++ {
		b := make([]byte, payloadLen)
		for i := 0; i < len(b); i++ {
			b[i] = 'a'
		}
		if err := codec.Encode(b, buf); err != nil {
			t.Fatal(err)
		}

		if buf.WriteLen() != HeaderLen+payloadLen {
			t.Fatal("invalid write length")
		}

		buf.Commit(buf.WriteLen())
		if buf.WriteLen() != 0 {
			t.Fatal("something very wrong going on with the byte buffer")
		}
		if buf.ReadLen() != HeaderLen+payloadLen {
			t.Fatal("invalid read length")
		}

		actualPayloadLen := binary.BigEndian.Uint32(buf.Data()[:HeaderLen])
		if int(actualPayloadLen) != payloadLen /* expected */ {
			t.Fatalf(
				"invalid encoded payload length actual=%d expected=%d",
				actualPayloadLen, payloadLen)
		}

		buf.Consume(HeaderLen)
		if len(buf.Data()) != buf.ReadLen() {
			t.Fatal("something very wrong going on with the byte buffer")
		}
		if buf.ReadLen() != payloadLen {
			t.Fatal("invalid payload length")
		}

		buf.Consume(payloadLen)
		if len(buf.Data()) != buf.ReadLen() {
			t.Fatal("something very wrong going on with the byte buffer")
		}
		if buf.ReadLen() != 0 {
			t.Fatal("something very wrong going on with the byte buffer")
		}
		if buf.WriteLen() != 0 {
			t.Fatal("something very wrong going on with the byte buffer")
		}
		if buf.Len() != 0 {
			t.Fatal("something very wrong going on with the byte buffer")
		}
	}
}

func TestFrameEncodeDecodeOne(t *testing.T) {
	buf := sonic.NewByteBuffer() /* we encode to/decode from the same buffer */
	codec := NewCodec(buf)
	if err := codec.Encode([]byte("hello, world!"), buf); err != nil {
		t.Fatal(err)
	}

	if b, err := codec.Decode(buf); err != nil {
		t.Fatal(err)
	} else if string(b) != "hello, world!" {
		t.Fatal("wrong encoding")
	}

	if !codec.decodeReset {
		t.Fatal("decoder should be marked to reset")
	}
	if codec.decodeBytes != len("hello, world!") {
		t.Fatalf("decoder decodeBytes should be %d but is %d",
			codec.decodeBytes, len("hello, world!"))
	}
}

func TestFrameEncodeDecodeTwo(t *testing.T) {
	for i := 0; i < 2; i++ {
		buf := sonic.NewByteBuffer() /* we encode to/decode from the same buffer */
		codec := NewCodec(buf)
		if err := codec.Encode([]byte("hello, world!"), buf); err != nil {
			t.Fatal(err)
		}

		if b, err := codec.Decode(buf); err != nil {
			t.Fatal(err)
		} else if string(b) != "hello, world!" {
			t.Fatal("wrong encoding")
		}

		if !codec.decodeReset {
			t.Fatal("decoder should be marked to reset")
		}
		if codec.decodeBytes != len("hello, world!") {
			t.Fatalf("decoder decodeBytes should be %d but is %d",
				codec.decodeBytes, len("hello, world!"))
		}
	}
}

func TestZeroPayload(t *testing.T) {
	buf := sonic.NewByteBuffer() /* we encode to/decode from the same buffer */
	codec := NewCodec(buf)

	header := make([]byte, HeaderLen)
	binary.BigEndian.PutUint32(header, 0)
	buf.Write(header)
	b, err := codec.Decode(buf)

	if len(b) != 0 {
		t.Fatal("payload should be 0")
	}
	if err != nil {
		t.Fatal("should have not gotten an error")
	}

	if !codec.decodeReset {
		t.Fatal("decode should be marked to reset")
	}
	if codec.decodeBytes != 0 {
		t.Fatal("0 bytes should be consumed on the next decode")
	}
}

func TestPartial(t *testing.T) {
	buf := sonic.NewByteBuffer()
	codec := NewCodec(buf)

	// read empty buffer
	{
		b, err := codec.Decode(buf)
		if b != nil {
			t.Fatal("shouldn't have gotten anything")
		}
		if err != sonicerrors.ErrNeedMore {
			t.Fatal("should have gotten ErrNeedMore")
		}

		if codec.decodeReset {
			t.Fatal("decode should not be reset")
		}
		if codec.decodeBytes != 0 {
			t.Fatal("decode bytes should be zero")
		}
	}

	// encode the header only, payload_len = 16
	{
		header := make([]byte, HeaderLen)
		binary.BigEndian.PutUint32(header, 16)
		buf.Write(header)
		b, err := codec.Decode(buf)
		if b != nil {
			t.Fatal("shouldn't have gotten anything")
		}
		if err != sonicerrors.ErrNeedMore {
			t.Fatal("should have gotten ErrNeedMore")
		}

		if codec.decodeReset {
			t.Fatal("decode should not be reset")
		}
		if codec.decodeBytes != 0 {
			t.Fatal("decode bytes should be zero")
		}
	}

	// encode 8 payload bytes
	{
		buf.Write([]byte("12345678"))
		b, err := codec.Decode(buf)
		if b != nil {
			t.Fatal("shouldn't have gotten anything")
		}
		if err != sonicerrors.ErrNeedMore {
			t.Fatal("should have gotten ErrNeedMore")
		}

		if codec.decodeReset {
			t.Fatal("decode should not be reset")
		}
		if codec.decodeBytes != 0 {
			t.Fatal("decode bytes should be zero")
		}
	}

	// encode the remaining 8 bytes
	{
		buf.Write([]byte("12345678"))
		b, err := codec.Decode(buf)
		if err != nil {
			t.Fatalf("got err=%v", err)
		}
		if string(b) != "1234567812345678" {
			t.Fatal("wrong payload")
		}

		if !codec.decodeReset {
			t.Fatal("decode should be marked to reset")
		}
		if codec.decodeBytes != 16 {
			t.Fatal("16 bytes should be consumed on the next decode")
		}
	}
}

func TestFrameEncodeDecodeRandom(t *testing.T) {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	fnMakeRandString := func(n int) string {
		b := make([]byte, n)
		for i := range b {
			b[i] = letters[rand.Int63()%int64(len(letters))]
		}
		return string(b)
	}

	buf := sonic.NewByteBuffer() /* we encode to/decode from the same buffer */
	codec := NewCodec(buf)
	for n := 0; n < 1024*32; n++ {
		toEncode := fnMakeRandString(n)
		if err := codec.Encode([]byte(toEncode), buf); err != nil {
			t.Fatal(err)
		}

		if b, err := codec.Decode(buf); err != nil {
			t.Fatal(err)
		} else if string(b) != toEncode {
			t.Fatalf("wrong encoding for length n=%d", n)
		}
	}
}
