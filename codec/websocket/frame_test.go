package websocket

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/talostrading/sonic"
)

type partialFrameWriter struct {
	b       [1024]byte
	invoked int
}

func (w *partialFrameWriter) Write(b []byte) (n int, err error) {
	copy(w.b[w.invoked:w.invoked+1], b)
	w.invoked++
	return 1, nil
}

func (w *partialFrameWriter) Bytes() []byte {
	return w.b[:w.invoked]
}

func TestFramePartialWrites(t *testing.T) {
	assert := assert.New(t)

	payload := make([]byte, 126)
	for i := 0; i < len(payload); i++ {
		payload[i] = 0
	}
	copy(payload, []byte("something"))

	f := NewFrame()
	f.SetFIN().SetText().SetPayload(payload)
	assert.Equal(2, f.ExtendedPayloadLengthBytes())
	assert.Equal(126, f.PayloadLength())

	w := &partialFrameWriter{}
	f.WriteTo(w)
	assert.Equal(2 /* mandatory flags */ +2 /* 2 bytes for the extended payload length */ +126 /* payload */, w.invoked)
	assert.Equal(130, len(w.Bytes()))

	// deserialize to make sure the frames are identical
	{
		f := Frame(w.Bytes())
		assert.True(f.IsFIN())
		assert.True(f.Opcode().IsText())
		assert.Equal(126, f.PayloadLength())
		assert.Equal(2, f.ExtendedPayloadLengthBytes())
		assert.Equal("something", string(f.Payload()[:len("something")]))
		for i := len("something"); i < len(f.Payload()); i++ {
			assert.Equal(0, int(f.Payload()[i]))
		}
	}
}

func TestUnder126Frame(t *testing.T) {
	var (
		f   = NewFrame()
		raw = []byte{0x81, 5} // fin=1 opcode=1 (text) payload_len=5
	)
	raw = append(raw, genRandBytes(5)...)

	buf := bufio.NewReader(bytes.NewBuffer(raw))

	_, err := f.ReadFrom(buf)
	if err != nil {
		t.Fatal(err)
	}

	checkFrame(t, f, false, true, raw[2:])
}

func Test126Frame(t *testing.T) {
	var (
		f   = NewFrame()
		raw = []byte{0x81, 126, 0, 200}
	)
	raw = append(raw, genRandBytes(200)...)

	buf := bufio.NewReader(bytes.NewBuffer(raw))

	_, err := f.ReadFrom(buf)
	if err != nil {
		t.Fatal(err)
	}

	checkFrame(t, f, false, true, raw[4:])
}

func Test127Frame(t *testing.T) {
	var (
		f   = NewFrame()
		raw = []byte{0x81, 127, 0, 0, 0, 0, 0, 0x01, 0xFF, 0xFF}
	)
	raw = append(raw, genRandBytes(131071)...)

	buf := bufio.NewReader(bytes.NewBuffer(raw))

	_, err := f.ReadFrom(buf)
	if err != nil {
		t.Fatal(err)
	}

	checkFrame(t, f, false, true, raw[10:])
}

func TestWriteFrame(t *testing.T) {
	var (
		f       = NewFrame()
		payload = []byte("heloo")
	)

	f.SetFIN()
	f.SetPayload(payload)
	f.SetText()

	b := bytes.NewBuffer(nil)
	f.WriteTo(b)

	under := b.Bytes()
	if under[0]&0x81 != under[0] {
		t.Fatal("expected fin and text set in first byte of buffer header")
	}

	if under[1]|0x05 != under[1] {
		t.Fatalf("expected length=5")
	}

	if len(under[2:]) != 5 {
		t.Fatalf("expected frame payload length=5")
	}

	if !bytes.Equal(under[2:], payload) {
		t.Fatalf("payload is not the same; given=%s expected=%s", under[2:], payload)
	}
}

func TestSameFrameWriteRead(t *testing.T) {
	var (
		header  = []byte{0x81, 5}
		payload = genRandBytes(5)
		buf     = sonic.NewByteBuffer()
		f       = NewFrame()
	)

	buf.Write(header)
	buf.Write(payload)
	buf.Commit(7)

	n, err := f.ReadFrom(buf)
	if err != nil {
		t.Fatal(err)
	}
	if n != 7 {
		t.Fatalf("frame is corrupt")
	}
	if !(f.IsFIN() && f.Opcode().IsText() && f.PayloadLength() == 5 && bytes.Equal(f.Payload(), payload)) {
		t.Fatalf("invalid frame")
	}

	buf.Consume(7)

	// serialize
	n, err = f.WriteTo(buf)
	if err != nil {
		t.Fatal(err)
	}
	if n != 7 {
		t.Fatalf("short frame write")
	}

	if buf.WriteLen() != 7 {
		t.Fatalf("BytesBuffer is not working properly")
	}
	buf.Commit(7)

	raw := append(header, payload...)
	if !bytes.Equal(raw, buf.Data()) {
		t.Fatalf("wrong frame serialization")
	}
}

func checkFrame(t *testing.T, f Frame, c, fin bool, payload []byte) {
	if c && !f.Opcode().IsContinuation() {
		t.Fatal("expected continuation")
	}

	if fin && !f.IsFIN() {
		t.Fatal("expected FIN")
	}

	if given, expected := len(payload), f.PayloadLength(); given != expected {
		t.Fatalf("invalid payload length; given=%d expected=%d", given, expected)
	}

	if p := f.Payload(); !bytes.Equal(p, payload) {
		t.Fatalf("invalid payload; given=%s expected=%s", p, payload)
	}
}

func genRandBytes(n int) []byte {
	b := make([]byte, n)
	rand.Read(b)
	return b
}
