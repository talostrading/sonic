package http

import (
	"net/http"

	"github.com/talostrading/sonic"
)

var (
	_ sonic.Codec[*http.Request, *http.Response] = &ClientCodec{}
	_ sonic.Codec[*http.Response, *http.Request] = &ServerCodec{}
)

type ServerCodec struct {
	src *sonic.ByteBuffer
	dst *sonic.ByteBuffer
}

func NewServerCodec(src, dst *sonic.ByteBuffer) *ServerCodec {
	c := &ServerCodec{
		src: src,
		dst: dst,
	}
	return c
}

func (c *ServerCodec) Decode(src *sonic.ByteBuffer) (*http.Request, error) {
	return nil, nil
}

func (c *ServerCodec) Encode(req *http.Response, dst *sonic.ByteBuffer) error {
	return nil
}
