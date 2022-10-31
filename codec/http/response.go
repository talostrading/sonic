package http

import (
	"bytes"
	"fmt"
	"github.com/talostrading/sonic"
	"strconv"
)

type Response struct {
	Proto      Proto
	StatusCode int
	Status     string
	Header     Header
	Body       []byte
}

func NewResponse() (*Response, error) {
	header, err := NewHeader()
	if err != nil {
		return nil, err
	}

	r := &Response{
		Header: header,
	}
	return r, nil
}

func (r *Response) Reset() {
	r.StatusCode = 0
	r.Status = ""
	r.Proto = ""
	r.Header.Reset()
	r.Body = nil
}

func DecodeResponseLine(line []byte, into *Response) (err error) {
	var statusCode int64

	line = bytes.TrimSpace(line)
	tokens := bytes.Fields(line)
	if len(tokens) < 2 {
		return &ResponseError{reason: "invalid response line", raw: line}
	}

	into.Proto, err = ParseProtoFromBytes(tokens[0])
	if err != nil {
		return &ResponseError{reason: fmt.Sprintf("invalid method err=%v", err), raw: line}
	}

	statusCode, err = strconv.ParseInt(string(tokens[1]), 10, 64)
	if err != nil {
		return &ResponseError{reason: fmt.Sprintf("invalid URI err=%v", err), raw: line}
	}
	into.StatusCode = int(statusCode)

	// TODO not the nicest
	into.Status = string(bytes.Join(tokens[2:], []byte(" ")))

	return nil
}

func EncodeResponseLine(res *Response, dst *sonic.ByteBuffer) error {
	dst.WriteString(res.Proto.String())
	dst.WriteString(" ")

	dst.WriteString(strconv.FormatInt(int64(res.StatusCode), 10))
	dst.WriteString(" ")

	dst.WriteString(res.Status)
	dst.WriteString(" ")

	dst.WriteString(CLRF)

	return nil
}

func ValidateResponse(res *Response) error {
	if res.Proto == "" {
		return ErrMissingProto
	}
	if res.Status == "" || res.StatusCode == 0 {
		return ErrMissingStatus
	}
	if ExpectBody(res.Header) && res.Body == nil {
		return ErrMissingBody
	}
	return nil
}
