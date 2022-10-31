package http

import (
	"errors"
	"fmt"
)

var (
	ErrInvalidHeader = errors.New("invalid header")
	ErrMissingMethod = errors.New("missing http method")
	ErrMissingURL    = errors.New("missing http url")
	ErrMissingProto  = errors.New("missing http protocol")
	ErrMissingBody   = errors.New("missing http body")
	ErrMissingStatus = errors.New("missing http status")
)

type RequestError struct {
	raw    []byte
	reason string
}

func (e *RequestError) Error() string {
	return fmt.Sprintf("invalid request reason=%s raw_request=%s", e.reason, string(e.raw))
}

type ResponseError struct {
	raw    []byte
	reason string
}

func (e *ResponseError) Error() string {
	return fmt.Sprintf("invalid response reason=%s raw_response=%s", e.reason, string(e.raw))
}
