package sonic

import (
	"errors"
	"github.com/talostrading/sonic/sonicerrors"
)

type qConn struct {
	*conn

	queue *Queue
}

func (qc *qConn) AsyncWrite(b []byte, cb AsyncCallback) {
	qc.file.writeReactor.init(b, false, qc.cbFunc(cb))
	if qc.queue != nil {
		qc.queue.Enqueue(qc.file)
		qc.scheduleWriteFunc(0, cb, qc.onQueuedWrite)
	}
}

func (qc *qConn) cbFunc(cb AsyncCallback) func(err error, n int) {
	return func(err error, n int) {
		if err != nil {
			cb(err, n)
			return
		}
		buf := make([]byte, 1)
		for {
			_, err = qc.Read(buf)
			if err != nil {
				if errors.Is(err, sonicerrors.ErrWouldBlock) {
					continue
				}
			}
			break
		}
		cb(err, n)
	}
}

func (qc *qConn) onQueuedWrite(err error) {
	if err != nil {
		qc.queue.deregister(qc.file, err)
	}
	qc.queue.Dequeue(qc.file)
}
