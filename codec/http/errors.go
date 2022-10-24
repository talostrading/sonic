package http

import (
	"errors"
	"fmt"
)

var (
	ErrInvalidHeader = errors.New("invalid header")
)

type RequestError struct {
	raw    []byte
	reason string
}

func (e *RequestError) Error() string {
	return fmt.Sprintf("invalid request reason=%s raw_request=%s", e.reason, string(e.raw))
}
