package sonic

import (
	"fmt"
	"io"
	"syscall"

	"github.com/talostrading/sonic/sonicopts"
)

var _ FileDescriptor = &AsyncAdapter{}

type AsyncAdapterHandler func(error, *AsyncAdapter)

// AsyncAdapter is a wrapper around syscall.Conn which enables
// clients to schedule async read and write operations on the
// underlying file descriptor.
type AsyncAdapter struct {
	*baseFd[*AsyncAdapter]

	rw io.ReadWriter
	rc syscall.RawConn
}

// NewAsyncAdapter takes in an IO instance and an interface of syscall.Conn and io.ReadWriter
// pertaining to the same object and invokes a completion handler which:
//   - provides the async adapter on successful completion
//   - provides an error if any occurred when async-adapting the provided object
//
// See async_adapter_test.go for examples on how to set up an AsyncAdapter.
func NewAsyncAdapter(
	ioc *IO,
	sc syscall.Conn,
	rw io.ReadWriter,
	cb AsyncAdapterHandler,
	opts ...sonicopts.Option,
) {
	rc, err := sc.SyscallConn()
	if err != nil {
		cb(err, nil)
		return
	}

	err = rc.Control(func(fd uintptr) {
		var (
			err     error
			adapter *AsyncAdapter

			rawFd = int(fd)
		)

		for _, opt := range opts {
			if opt.Type() == sonicopts.TypeNonblocking {
				err = fmt.Errorf("option nonblocking not supported for AsyncAdapter")
				break
			}
		}

		if err == nil {
			adapter = &AsyncAdapter{
				rw: rw,
				rc: rc,
			}
			adapter.baseFd, err = newBaseFd(ioc, rawFd, adapter, opts...)
		}

		cb(err, adapter)
	})

	if err != nil {
		cb(err, nil)
	}
}

func (a *AsyncAdapter) Read(b []byte) (int, error) {
	return a.rw.Read(b)
}

func (a *AsyncAdapter) Write(b []byte) (int, error) {
	return a.rw.Write(b)
}

func (a *AsyncAdapter) AsyncRead(b []byte, cb AsyncCallback) {
	a.scheduleRead(b, 0, false, cb)
}

func (a *AsyncAdapter) AsyncReadAll(b []byte, cb AsyncCallback) {
	a.scheduleRead(b, 0, true, cb)
}

func (a *AsyncAdapter) AsyncWrite(b []byte, cb AsyncCallback) {
	a.scheduleWrite(b, 0, false, cb)
}

func (a *AsyncAdapter) AsyncWriteAll(b []byte, cb AsyncCallback) {
	a.scheduleWrite(b, 0, true, cb)
}
