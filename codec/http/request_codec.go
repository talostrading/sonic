package http

import (
	"bytes"
	"fmt"
	"github.com/talostrading/sonic"
	"net/url"
	"strconv"
)

var _ sonic.Codec[*Request, *Request] = &RequestCodec{}

type decodeState uint8

const (
	stateRequestLine decodeState = iota
	stateHeader
	stateBody
	stateDone
)

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
		decodeState: stateRequestLine,
		decodeReq:   req,
	}

	return c, nil
}

func (c *RequestCodec) Encode(req *Request, dst *sonic.ByteBuffer) error {
	// encode request-line
	dst.WriteString(req.Method.String())
	dst.WriteString(" ")

	dst.WriteString(req.URL.String())
	dst.WriteString(" ")

	dst.WriteString(req.Proto.String())
	dst.WriteString(" ")

	dst.WriteString(CLRF)

	// encode headers
	req.Header.WriteTo(dst)

	// encode body
	dst.Write(req.Body)

	return nil
}

func (c *RequestCodec) resetDecode() {
	if c.decodeState == stateDone {
		c.decodeState = stateRequestLine
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
	case stateRequestLine:
		goto decodeRequestLine
	case stateHeader:
		goto decodeHeader
	case stateBody:
		goto decodeBody
	case stateDone:
		goto done
	default:
		panic(fmt.Errorf("unhandled state %d", s))
	}

decodeRequestLine:
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
			if c.decodeReq.Header.Has("Content-Length") || c.decodeReq.Header.Has("Transfer-Encoding") {
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

func DecodeRequestLine(line []byte, into *Request) (err error) {
	line = bytes.TrimSpace(line)
	tokens := bytes.Fields(line)
	if len(tokens) != 3 {
		return &RequestError{reason: "invalid request line", raw: line}
	}

	into.Method, err = ParseMethodFromBytes(tokens[0])
	if err != nil {
		return &RequestError{reason: fmt.Sprintf("invalid method err=%v", err), raw: line}
	}

	into.URL, err = url.Parse(string(tokens[1]))
	if err != nil {
		return &RequestError{reason: fmt.Sprintf("invalid URI err=%v", err), raw: line}
	}

	into.Proto, err = ParseProtoFromBytes(tokens[2])
	if err != nil {
		return &RequestError{reason: fmt.Sprintf("invalid http protocol err=%v", err), raw: line}
	}

	return nil
}
