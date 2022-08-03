package http

import (
	"bufio"
	"io"
	"net/http"
	"net/http/httputil"

	"github.com/talostrading/sonic"
	"github.com/talostrading/sonic/sonicerrors"
)

var (
	_ sonic.Codec[*http.Request, *http.Response] = &ClientCodec{}
	_ sonic.Codec[*http.Response, *http.Request] = &ServerCodec{}
)

type ClientCodec struct {
	src *sonic.ByteBuffer
	dst *sonic.ByteBuffer

	br *bufio.Reader

	lastReq *http.Request
}

func NewClientCodec(src, dst *sonic.ByteBuffer) *ClientCodec {
	c := &ClientCodec{
		src: src,
		dst: dst,
		br:  bufio.NewReader(src),
	}
	return c
}

func (c *ClientCodec) Decode(src *sonic.ByteBuffer) (*http.Response, error) {
	c.src.PrepareRead(c.src.WriteLen())

	res, err := http.ReadResponse(c.br, c.lastReq)
	if err == io.ErrUnexpectedEOF {
		return nil, sonicerrors.ErrNeedMore
	}

	return res, err
}

func (c *ClientCodec) Encode(req *http.Request, dst *sonic.ByteBuffer) error {
	c.lastReq = req

	b, err := httputil.DumpRequest(req, true)
	if err != nil {
		return err
	}

	n, err := c.dst.Write(b)
	if err != nil {
		return err
	}

	c.dst.Commit(n)

	return nil
}
