package http

import (
	"fmt"
	"github.com/talostrading/sonic"
	"strconv"
)

var _ sonic.Codec[*Request, *Request] = &RequestCodec{}

type RequestCodec struct {
	decodeState decodeState
	decodeReq   *Request // request we decode into
}

func NewRequestCodec() (*RequestCodec, error) {
	req, err := NewRequest()
	if err != nil {
		return nil, err
	}

	c := &RequestCodec{
		decodeState: stateFirstLine,
		decodeReq:   req,
	}

	return c, nil
}

func (c *RequestCodec) Encode(req *Request, dst *sonic.ByteBuffer) error {
	if err := ValidateRequest(req); err != nil {
		return err
	}

	// status-line
	if err := EncodeRequestLine(req, dst); err != nil {
		return err
	}

	// header
	if _, err := req.Header.WriteTo(dst); err != nil {
		return err
	}

	// body
	if ExpectBody(req.Header) {
		if n, err := dst.Write(req.Body); err != nil || n != len(req.Body) {
			return err
		}
	}

	return nil
}

func (c *RequestCodec) resetDecode() {
	if c.decodeState == stateDone {
		c.decodeState = stateFirstLine
		c.decodeReq.Reset()
	}
}

func (c *RequestCodec) Decode(src *sonic.ByteBuffer) (*Request, error) {
	c.resetDecode()

	var (
		line []byte
		err  error

		headerKey, headerValue []byte

		n int64
	)

prepareDecode:
	if err != nil {
		goto done
	}

	switch s := c.decodeState; s {
	case stateFirstLine:
		goto decodeFirstLine
	case stateHeader:
		goto decodeHeader
	case stateBody:
		goto decodeBody
	case stateDone:
		goto done
	default:
		panic(fmt.Errorf("unhandled state %d", s))
	}

decodeFirstLine:
	line, err = src.NextLine()
	if err == nil {
		err = DecodeRequestLine(line, c.decodeReq)
	}

	if err == nil {
		c.decodeState = stateHeader
	} else {
		c.decodeState = stateDone
	}
	goto prepareDecode

decodeHeader:
	line, err = src.NextLine()
	if err == nil {
		if len(line) == 0 {
			// CLRF - end of header
			if ExpectBody(c.decodeReq.Header) {
				c.decodeState = stateBody
			} else {
				c.decodeState = stateDone
			}
		} else {
			headerKey, headerValue, err = DecodeHeaderLine(line)
			if err == nil {
				c.decodeReq.Header.Add(string(headerKey), string(headerValue))
			}
		}
	}
	goto prepareDecode

decodeBody:
	n, err = strconv.ParseInt(c.decodeReq.Header.Get("Content-Length"), 10, 64)
	if err == nil {
		err = src.PrepareRead(int(n))
		if err == nil {
			c.decodeReq.Body = src.Data()
			c.decodeState = stateDone
		}
	}
	goto prepareDecode

done:
	return c.decodeReq, err
}
