package http

import (
	"bytes"
	"io"
)

var _ Header = mapHeader{}

func NewHeader() (Header, error) {
	var h mapHeader = make(map[string]string)
	return h, nil
}

type mapHeader map[string]string

func (h mapHeader) Add(key, value string) {
	h[key] = value
}

func (h mapHeader) Set(key, value string) {
	if _, ok := h[key]; ok {
		h[key] = value
	}
}

func (h mapHeader) Get(key string) string {
	return h[key]
}

func (h mapHeader) Del(key string) {
	delete(h, key)
}

func (h mapHeader) WriteTo(w io.Writer) (int64, error) {
	var b []byte
	for k, v := range h {
		b = append(b, k...)
		b = append(b, ": "...)
		b = append(b, v...)
		b = append(b, CLRF...)
	}
	n, err := w.Write(b)
	return int64(n), err
}

func (h mapHeader) Has(key string) bool {
	_, ok := h[key]
	return ok
}

func (h mapHeader) Len() int {
	return len(h)
}

func (h mapHeader) Reset() {
	for k := range h {
		delete(h, k)
	}
}

func DecodeHeaderLine(line []byte) (key, value []byte, err error) {
	if i := bytes.IndexByte(line, ':'); i >= 0 {
		key = bytes.TrimSpace(line[:i])
		value = bytes.TrimSpace(line[i+1:])
	} else {
		err = ErrInvalidHeader
	}
	return
}

func ExpectBody(header Header) bool {
	return header.Has("Content-Length") || header.Has("Transfer-Encoding")
}
