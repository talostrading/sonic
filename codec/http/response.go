package http

import (
	"bytes"
	"fmt"
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
	if len(tokens) != 3 {
		return &RequestError{reason: "invalid request line", raw: line}
	}

	into.Proto, err = ParseProtoFromBytes(tokens[0])
	if err != nil {
		return &RequestError{reason: fmt.Sprintf("invalid method err=%v", err), raw: line}
	}

	statusCode, err = strconv.ParseInt(string(tokens[1]), 10, 64)
	if err != nil {
		return &RequestError{reason: fmt.Sprintf("invalid URI err=%v", err), raw: line}
	}
	into.StatusCode = int(statusCode)

	into.Status = string(tokens[2])

	return nil
}
