package http

import (
	"fmt"
	"github.com/talostrading/sonic"
	"strconv"
)

var _ sonic.Codec[*Response, *Response] = &ResponseCodec{}

type ResponseCodec struct {
	decodeState decodeState
	decodeRes   *Response
}

func NewResponseCodec() (*ResponseCodec, error) {
	res, err := NewResponse()
	if err != nil {
		return nil, err
	}

	c := &ResponseCodec{
		decodeState: stateFirstLine,
		decodeRes:   res,
	}
	return c, nil
}

func (c *ResponseCodec) Encode(res *Response, dst *sonic.ByteBuffer) error {
	if err := ValidateResponse(res); err != nil {
		return err
	}

	// status-line
	if err := EncodeResponseLine(res, dst); err != nil {
		return err
	}

	// header
	if _, err := res.Header.WriteTo(dst); err != nil {
		return err
	}

	// body
	if ExpectBody(res.Header) {
		if n, err := dst.Write(res.Body); err != nil || n != len(res.Body) {
			return err
		}
	}

	return nil
}

func (c *ResponseCodec) resetDecode() {
	if c.decodeState == stateDone {
		c.decodeRes.Reset()
		c.decodeState = stateFirstLine
	}
}

func (c *ResponseCodec) Decode(src *sonic.ByteBuffer) (*Response, error) {
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
		err = DecodeResponseLine(line, c.decodeRes)
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
			if ExpectBody(c.decodeRes.Header) {
				c.decodeState = stateBody
			} else {
				c.decodeState = stateDone
			}
		} else {
			headerKey, headerValue, err = DecodeHeaderLine(line)
			if err == nil {
				c.decodeRes.Header.Add(string(headerKey), string(headerValue))
			}
		}
	}
	goto prepareDecode

decodeBody:
	n, err = strconv.ParseInt(c.decodeRes.Header.Get("Content-Length"), 10, 64)
	if err == nil {
		err = src.PrepareRead(int(n))
		if err == nil {
			c.decodeRes.Body = src.Data()
			c.decodeState = stateDone
		}
	}
	goto prepareDecode

done:
	return c.decodeRes, err
}
