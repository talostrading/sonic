package sonic

import "errors"

type AsyncCallback func(error, int)

type File interface {
	Read([]byte) (int, error)
	Write([]byte) (int, error)

	// AsyncRead tries to read as much as possible into the provided buffer
	AsyncRead([]byte, AsyncCallback)

	// AsyncWrite tries to write as much as possible from the provided buffer
	AsyncWrite([]byte, AsyncCallback)

	// AsyncReadAll completes when the provided buffer is full
	AsyncReadAll([]byte, AsyncCallback)

	// AsyncWriteAll completes when the provided buffer is full
	AsyncWriteAll([]byte, AsyncCallback)

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
