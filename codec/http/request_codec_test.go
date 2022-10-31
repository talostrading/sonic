package http

import (
	"bufio"
	"bytes"
	"github.com/talostrading/sonic"
	"github.com/talostrading/sonic/sonicerrors"
	"net/url"
	"testing"
)

func TestRequestCodec_EncodeValidRequest(t *testing.T) {
	codec, err := NewRequestCodec()
	if err != nil {
		t.Fatal(err)
	}

	req, err := NewRequest()
	if err != nil {
		t.Fatal(err)
	}

	req.Method = Get
	req.URL, err = url.Parse("http://localhost:8080")
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Add("Content-Length", "10")
	req.Body = []byte("0123456789")
	req.Proto = ProtoHttp11

	dst := sonic.NewByteBuffer()
	err = codec.Encode(req, dst)
	if err != nil {
		t.Fatal(err)
	}

	dst.Commit(1000)

	rd := bufio.NewReader(dst)

	line, _, err := rd.ReadLine()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(bytes.TrimSpace(line), []byte("GET http://localhost:8080 HTTP/1.1")) {
		t.Fatal("wrong request line")
	}

	line, _, err = rd.ReadLine()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(bytes.TrimSpace(line), []byte("Content-Length: 10")) {
		t.Fatal("wrong header")
	}

	line, _, err = rd.ReadLine()
	if err != nil {
		t.Fatal(err)
	}
	if len(line) != 0 {
		t.Fatal("wrong header ending")
	}

	body := make([]byte, 128)
	n, err := rd.Read(body)
	if n != 10 || err != nil {
		t.Fatal("wrong body")
	}
	if string(body[:n]) != "0123456789" {
		t.Fatal("wrong body")
	}
}

func TestRequestCodec_EncodeInvalidRequest(t *testing.T) {
	req, err := NewRequest()
	if err != nil {
		t.Fatal(err)
	}

	codec, err := NewRequestCodec()
	if err != nil {
		t.Fatal(err)
	}

	err = codec.Encode(req, sonic.NewByteBuffer())
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestRequestCodec_DecodeValidRequest(t *testing.T) {
	codec, err := NewRequestCodec()
	if err != nil {
		t.Fatal(err)
	}

	b := sonic.NewByteBuffer()
	b.WriteString("GET /host HTTP/1.1\r\n")
	b.WriteString("Content-Length: 10\r\n")
	b.WriteString("Something: 10\r\n")
	b.WriteString("\r\n")
	body := "0123456789"
	b.WriteString(body)

	req, err := codec.Decode(b)
	if err != nil {
		t.Fatal(err)
	}

	if req.Method != Get {
		t.Fatal("wrong method")
	}
	if req.URL.String() != "/host" {
		t.Fatal("wrong URI")
	}
	if req.Proto != ProtoHttp11 {
		t.Fatal("wrong proto")
	}
	if req.Header.Get("Content-Length") != "10" {
		t.Fatal("wrong header")
	}
	if req.Header.Get("Something") != "10" {
		t.Fatal("wrong header")
	}
	if req.Header.Len() != 2 {
		t.Fatal("wrong header")
	}
	if string(req.Body) != body {
		t.Fatal("wrong body")
	}
}

func TestRequestCodec_InvalidRequest(t *testing.T) {
	codec, err := NewRequestCodec()
	if err != nil {
		t.Fatal(err)
	}

	b := sonic.NewByteBuffer()
	b.WriteString("GET /host HTTP/1.1\r\n")
	b.WriteString("sdflkjadlskfjalsdfj\r\n")

	_, err = codec.Decode(b)
	if err != ErrInvalidHeader {
		t.Fatal("expected ErrInvalidHeader")
	}
}

func TestRequestCodec_DecodeRequestPartByPart(t *testing.T) {
	codec, err := NewRequestCodec()
	if err != nil {
		t.Fatal(err)
	}

	b := sonic.NewByteBuffer()
	if codec.decodeState != stateFirstLine {
		t.Fatal("wrong decoder state")
	}

	b.WriteString("GET /host HTTP/1.1\r\n")
	_, err = codec.Decode(b)
	if err != sonicerrors.ErrNeedMore {
		t.Fatal("should have gotten ErrNeedMore")
	}
	if codec.decodeState != stateHeader {
		t.Fatal("wrong decoder state")
	}

	b.WriteString("Content-Length: 10\r\n")
	_, err = codec.Decode(b)
	if err != sonicerrors.ErrNeedMore {
		t.Fatal("should have gotten ErrNeedMore")
	}
	if codec.decodeState != stateHeader {
		t.Fatal("wrong decoder state")
	}

	b.WriteString("Something: 10\r\n")
	_, err = codec.Decode(b)
	if err != sonicerrors.ErrNeedMore {
		t.Fatal("should have gotten ErrNeedMore")
	}
	if codec.decodeState != stateHeader {
		t.Fatal("wrong decoder state")
	}

	b.WriteString("\r\n")
	_, err = codec.Decode(b)
	if err != sonicerrors.ErrNeedMore {
		t.Fatal("should have gotten ErrNeedMore")
	}
	if codec.decodeState != stateBody {
		t.Fatal("wrong decoder state")
	}

	body := "0123456789"
	b.WriteString(body)
	req, err := codec.Decode(b)
	if err != nil {
		t.Fatal(err)
	}
	if req.Method != Get {
		t.Fatal("wrong method")
	}
	if req.URL.String() != "/host" {
		t.Fatal("wrong URI")
	}
	if req.Proto != ProtoHttp11 {
		t.Fatal("wrong proto")
	}
	if req.Header.Get("Content-Length") != "10" {
		t.Fatal("wrong header")
	}
	if req.Header.Get("Something") != "10" {
		t.Fatal("wrong header")
	}
	if req.Header.Len() != 2 {
		t.Fatal("wrong header")
	}
	if string(req.Body) != body {
		t.Fatal("wrong body")
	}
}
