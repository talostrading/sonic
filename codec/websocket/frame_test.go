package websocket

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"testing"

	"github.com/talostrading/sonic"
)

func TestUnder125Frame(t *testing.T) {
	raw := []byte{0x81, 5} // fin=1 opcode=1 (text) payload_len=5
	raw = append(raw, genRandBytes(5)...)

	f := AcquireFrame()
	defer ReleaseFrame(f)

	buf := bufio.NewReader(bytes.NewBuffer(raw))

	_, err := f.ReadFrom(buf)
	if err != nil {
		t.Fatal(err)
	}

	checkFrame(t, f, false, true, raw[2:])
}

func Test126Frame(t *testing.T) {
	raw := []byte{0x81, 126, 0, 200}
	raw = append(raw, genRandBytes(200)...)

	f := AcquireFrame()
	defer ReleaseFrame(f)

	buf := bufio.NewReader(bytes.NewBuffer(raw))

	_, err := f.ReadFrom(buf)
	if err != nil {
		t.Fatal(err)
	}

	checkFrame(t, f, false, true, raw[4:])
}

func Test127Frame(t *testing.T) {
	raw := []byte{0x81, 127, 0, 0, 0, 0, 0, 0x01, 0xFF, 0xFF}
	raw = append(raw, genRandBytes(131071)...)

	f := AcquireFrame()
	defer ReleaseFrame(f)

	buf := bufio.NewReader(bytes.NewBuffer(raw))

	_, err := f.ReadFrom(buf)
	if err != nil {
		t.Fatal(err)
	}

	checkFrame(t, f, false, true, raw[10:])
}

func TestWriteFrame(t *testing.T) {
	payload := []byte("heloo")

	f := AcquireFrame()
	defer ReleaseFrame(f)

	f.SetFin()
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
	// deserialize
	f := AcquireFrame()
	defer ReleaseFrame(f)

	header := []byte{0x81, 5}
	payload := genRandBytes(5)

	buf := sonic.NewBytesBuffer()
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
	if !(f.IsFin() && f.IsText() && f.PayloadLen() == 5 && bytes.Equal(f.Payload(), payload)) {
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

func checkFrame(t *testing.T, f *Frame, c, fin bool, payload []byte) {
	if c && !f.IsContinuation() {
		t.Fatal("expected continuation")
	}

	if fin && !f.IsFin() {
		t.Fatal("expected FIN")
	}

	if given, expected := len(payload), f.PayloadLen(); given != expected {
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
