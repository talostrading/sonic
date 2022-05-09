package sonic

import "errors"

type AsyncCallback func(error, int)

type File interface {
	Read([]byte) (int, error)
	Write([]byte) (int, error)

	AsyncRead([]byte, AsyncCallback)
	AsyncWrite([]byte, AsyncCallback)

	Close() error
}

var (
	ErrWouldBlock = errors.New("operation would block")
)
