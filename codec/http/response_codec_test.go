package http

import (
	"github.com/talostrading/sonic"
	"testing"
)

func TestResponseCodec_EncodeValidResponse(t *testing.T) {
	res, err := NewResponse()
	if err != nil {
		t.Fatal(err)
	}

	codec, err := NewResponseCodec()
	if err != nil {
		t.Fatal(err)
	}

	dst := sonic.NewByteBuffer()

	res.Proto = ProtoHttp11
	res.StatusCode = 200
	res.Status = "200 OK"

	if err := codec.Encode(res, dst); err != nil {
		t.Fatal(err)
	}
}

func TestResponseCodec_EncodeInvalidResponse(t *testing.T) {
	res, err := NewResponse()
	if err != nil {
		t.Fatal(err)
	}

	codec, err := NewResponseCodec()
	if err != nil {
		t.Fatal(err)
	}

	dst := sonic.NewByteBuffer()

	if err := codec.Encode(res, dst); err == nil {
		t.Fatal("expected error")
	}
}

func TestResponseCodec_Decode(t *testing.T) {
	codec, err := NewResponseCodec()
	if err != nil {
		t.Fatal(err)
	}

	src := sonic.NewByteBuffer()

	src.WriteString("HTTP/1.1 200 OK\r\n")
	src.WriteString(CLRF)

	res, err := codec.Decode(src)
	if err != nil {
		t.Fatal(err)
	}

	if res.Proto != ProtoHttp11 {
		t.Fatal("invalid http protocol")
	}
	if res.StatusCode != 200 {
		t.Fatal("invalid status code")
	}
	if res.Status != StatusText(StatusOK) {
		t.Fatal("invalid reason-phrase")
	}
}
