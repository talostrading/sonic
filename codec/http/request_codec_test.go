package http

import (
	"fmt"
	"github.com/talostrading/sonic"
	"github.com/talostrading/sonic/sonicerrors"
	"testing"
)

func TestRequestCodec_EncodeValidRequest(t *testing.T) {
	// TODO
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
		fmt.Println(string(req.Body))
		t.Fatal("wrong body")
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
		fmt.Println(string(req.Body))
		t.Fatal("wrong body")
	}
}
