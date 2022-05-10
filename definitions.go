package sonic

import "errors"

type AsyncCallback func(error, int)

type File interface {
	Read([]byte) (int, error)
	Write([]byte) (int, error)

	AsyncRead([]byte, AsyncCallback)
	AsyncWrite([]byte, AsyncCallback)

	Seek(offset int64, whence SeekWhence) error

	Close() error
}

const (
	maxReadDispatch  int = 512
	maxWriteDispatch int = 512
)

var (
	ErrWouldBlock = errors.New("operation would block")
)

type SeekWhence int

const (
	SeekStart SeekWhence = iota
	SeekCurrent
	SeekEnd
)
