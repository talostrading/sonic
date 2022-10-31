package http

import (
	"bytes"
	"fmt"
	"github.com/talostrading/sonic"
	"net/url"
)

type Request struct {
	Method Method
	URL    *url.URL
	Proto  Proto
	Header Header
	Body   []byte
}

func NewRequest() (*Request, error) {
	header, err := NewHeader()
	if err != nil {
		return nil, err
	}

	r := &Request{
		Header: header,
	}
	return r, nil
}

func (r *Request) Reset() {
	r.Method = ""
	r.URL = nil
	r.Proto = ""
	r.Header.Reset()
	r.Body = nil
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

func EncodeRequestLine(req *Request, dst *sonic.ByteBuffer) error {
	dst.WriteString(req.Method.String())
	dst.WriteString(" ")

	dst.WriteString(req.URL.String())
	dst.WriteString(" ")

	dst.WriteString(req.Proto.String())
	dst.WriteString(" ")

	dst.WriteString(CLRF)

	return nil
}

func ValidateRequest(req *Request) error {
	if req.Method == "" {
		return ErrMissingMethod
	}
	if req.URL == nil {
		return ErrMissingURL
	}
	if req.Proto == "" {
		return ErrMissingProto
	}
	if ExpectBody(req.Header) && req.Body == nil {
		return ErrMissingBody
	}
	return nil
}
