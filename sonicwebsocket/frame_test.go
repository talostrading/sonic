package sonicwebsocket

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"testing"
)

func TestUnder125Frame(t *testing.T) {
	raw := []byte{0x81, 5}
	raw = append(raw, genRandBytes(5)...)

	fr := AcquireFrame()
	defer ReleaseFrame(fr)

	buf := bufio.NewReader(bytes.NewBuffer(raw))

	_, err := fr.ReadFrom(buf)
	if err != nil {
		t.Fatal(err)
	}

	checkFrame(t, fr, false, true, raw[2:])
}

func Test126Frame(t *testing.T) {
	raw := []byte{0x81, 126, 0, 200}
	raw = append(raw, genRandBytes(200)...)

	fr := AcquireFrame()
	defer ReleaseFrame(fr)

	buf := bufio.NewReader(bytes.NewBuffer(raw))

	_, err := fr.ReadFrom(buf)
	if err != nil {
		t.Fatal(err)
	}

	checkFrame(t, fr, false, true, raw[4:])
}

func Test127Frame(t *testing.T) {
	raw := []byte{0x81, 127, 0, 0, 0, 0, 0, 0x01, 0xFF, 0xFF}
	raw = append(raw, genRandBytes(131071)...)

	fr := AcquireFrame()
	defer ReleaseFrame(fr)

	buf := bufio.NewReader(bytes.NewBuffer(raw))

	_, err := fr.ReadFrom(buf)
	if err != nil {
		t.Fatal(err)
	}

	checkFrame(t, fr, false, true, raw[10:])
}

func TestWriteFrame(t *testing.T) {
	payload := []byte("heloo")

	fr := AcquireFrame()
	defer ReleaseFrame(fr)

	fr.SetFin()
	fr.SetPayload(payload)
	fr.SetText()

	b := bytes.NewBuffer(nil)
	fr.WriteTo(b)

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

func checkFrame(t *testing.T, fr *frame, c, fin bool, payload []byte) {
	if c && !fr.IsContinuation() {
		t.Fatal("expected continuation")
	}

	if fin && !fr.IsFin() {
		t.Fatal("expected FIN")
	}

	if len(payload) != int(fr.Len()) {
		t.Fatalf("invalid payload length; given=%d expected=%d", int(fr.Len()), len(payload))
	}

	if p := fr.Payload(); !bytes.Equal(p, payload) {
		t.Fatalf("invalid payload; given=%s expected=%s", p, payload)
	}
}

func genRandBytes(n int) []byte {
	b := make([]byte, n)
	rand.Read(b)
	return b
}
