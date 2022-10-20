package sonic

import (
	"fmt"
	"github.com/talostrading/sonic/sonicopts"
	"io"
	"syscall"
)

var _ FileDescriptor = &AsyncAdapter[AdapteeType]{}

// AdapteeType is the interface a type must implement in order to be async-adapted.
type AdapteeType interface {
	io.ReadWriter
}

// AsyncAdapter is a wrapper around syscall.Conn which enables
// clients to schedule async read and write operations on the
// underlying file descriptor.
type AsyncAdapter[T AdapteeType] struct {
	*baseFd[*AsyncAdapter[T]]
	adaptee T
}

// NewAsyncAdapter takes in an IO instance and an interface of syscall.Conn and io.ReadWriter
// pertaining to the same object and invokes a completion handler which:
//   - provides the async adapter on successful completion
//   - provides an error if any occurred when async-adapting the provided object
//
// See async_adapter_test.go for examples on how to set up an AsyncAdapter.
func NewAsyncAdapter[T AdapteeType](
	ioc *IO,
	adaptee T,
	opts ...sonicopts.Option,
) (adapter *AsyncAdapter[T], err error) {
	syscallConn, ok := interface{}(adaptee).(syscall.Conn)
	if !ok {
		return nil, fmt.Errorf("adaptee must implement syscall.Conn")
	}

	rawConn, err := syscallConn.SyscallConn()
	if err != nil {
		return nil, err
	}

	wait := make(chan struct{}, 1)
	err = rawConn.Control(func(fd uintptr) {
		for _, opt := range opts {
			if opt.Type() == sonicopts.TypeNonblocking {
				err = fmt.Errorf("option nonblocking not supported for AsyncAdapter")
				break
			}
		}

		if err == nil {
			rawFd := int(fd)

			adapter = &AsyncAdapter[T]{
				adaptee: adaptee,
			}
			adapter.baseFd, err = newBaseFd(ioc, rawFd, adapter, opts...)
		}

		wait <- struct{}{}
	})
	<-wait

	return
}

func (a *AsyncAdapter[T]) Read(b []byte) (int, error) {
	return a.adaptee.Read(b)
}

func (a *AsyncAdapter[T]) AsyncRead(b []byte, cb AsyncCallback) {
	a.scheduleRead(b, 0, false, cb)
}

func (a *AsyncAdapter[T]) AsyncReadAll(b []byte, cb AsyncCallback) {
	a.scheduleRead(b, 0, true, cb)
}

func (a *AsyncAdapter[T]) Write(b []byte) (int, error) {
	return a.adaptee.Write(b)
}

func (a *AsyncAdapter[T]) AsyncWrite(b []byte, cb AsyncCallback) {
	a.scheduleWrite(b, 0, false, cb)
}

func (a *AsyncAdapter[T]) AsyncWriteAll(b []byte, cb AsyncCallback) {
	a.scheduleWrite(b, 0, true, cb)
}
