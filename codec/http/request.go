package http

import (
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
