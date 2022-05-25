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
	raw := []byte{0x81, 127, 0, 0, 0, 0, 0, 0, 0xFF, 0xFF}
	raw = append(raw, genRandBytes(65535)...)

	fr := AcquireFrame()
	defer ReleaseFrame(fr)

	buf := bufio.NewReader(bytes.NewBuffer(raw))

	_, err := fr.ReadFrom(buf)
	if err != nil {
		t.Fatal(err)
	}

	checkFrame(t, fr, false, true, raw[10:])
}

func checkFrame(t *testing.T, fr *Frame, c, fin bool, payload []byte) {
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
