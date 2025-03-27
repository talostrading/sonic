package sonic

import (
	"github.com/talostrading/sonic/internal"
	"github.com/talostrading/sonic/sonicopts"
	"net"
	"time"
)

type Queue struct {
	order   []*file
	pending []bool
}

func NewQueue() *Queue {
	return &Queue{
		order:   make([]*file, 0),
		pending: make([]bool, 0),
	}
}

func (q *Queue) Enqueue(f *file) {
	q.order = append(q.order, f)
	q.pending = append(q.pending, false)
}

func (q *Queue) Dequeue(file *file) {
	if len(q.order) == 0 {
		return
	}
	if file != q.order[0] {
		q.suspend(file)
		q.executePending()
		return
	}
	q.dequeue()
	q.executePending()
	if len(q.order) == 0 {
		q.executePending()
		return
	}
}

func (q *Queue) dequeue() {
	f := q.order[0]
	f.writeReactor.onWrite(nil)
	time.Sleep(500 * time.Microsecond)
	q.order = q.order[1:]
	q.pending = q.pending[1:]
}

func (q *Queue) deregister(file *file, err error) {
	//TODO:add removal from que
	//TODO: why not use f.writeReactor.onWrite(err)
	file.ioc.Deregister(&file.writeReactor.file.slot)
	if err != nil {
		file.writeReactor.cb(err, file.writeReactor.wroteSoFar)
	}
}

func (q *Queue) executePending() {
	for {
		if len(q.pending) == 0 {
			break
		}
		if !q.pending[0] {
			break
		}
		q.dequeue()
		break
	}
}

func (q *Queue) suspend(f *file) {
	for i := 0; i < len(q.order); i++ {
		if q.order[i] != f {
			continue
		}
		q.pending[i] = true
		break
	}
}

func DialQ(
	ioc *IO,
	network, addr string, q *Queue,
	opts ...sonicopts.Option,
) (Conn, error) {
	return DialTimeoutQ(ioc, network, addr, 10*time.Second, q, opts...)
}

func DialTimeoutQ(
	ioc *IO, network, addr string,
	timeout time.Duration, q *Queue,
	opts ...sonicopts.Option,
) (Conn, error) {
	fd, localAddr, remoteAddr, err := internal.ConnectTimeout(network, addr, timeout, opts...)
	if err != nil {
		return nil, err
	}

	return newConnQ(ioc, fd, localAddr, remoteAddr, q), nil
}

func newConnQ(
	ioc *IO,
	fd int,
	localAddr, remoteAddr net.Addr, q *Queue,
) *conn {
	return &conn{
		file:       newFileQueue(ioc, fd, q),
		localAddr:  localAddr,
		remoteAddr: remoteAddr,
	}
}
