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
	if file == q.order[0] {
		q.dequeue()
	} else {
		q.suspend(file)
	}
	q.executePending()
}

func (q *Queue) dequeue() {
	f := q.order[0]
	f.writeReactor.onWrite(nil)
	q.order = q.order[1:]
	q.pending = q.pending[1:]
}

func (q *Queue) deregister(file *file, err error) {
	q.findFunc(file, func(q *Queue, index int) {
		removeAtIndex(q, index)
	})
	file.writeReactor.onWrite(err)
}

func removeAtIndex(q *Queue, index int) {
	if index < 0 || index >= len(q.order) {
		return
	}
	q.order = append(q.order[:index], q.order[index+1:]...)
	q.pending = append(q.pending[:index], q.pending[index+1:]...)
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
	}
}

func (q *Queue) suspend(f *file) {
	q.findFunc(f, func(q *Queue, index int) {
		q.pending[index] = true
	})
}

func (q *Queue) findFunc(f *file, consume func(q *Queue, index int)) {
	for i := 0; i < len(q.order); i++ {
		if q.order[i] != f {
			continue
		}
		consume(q, i)
		break
	}
}

func QDial(
	ioc *IO,
	network, addr string, q *Queue,
	opts ...sonicopts.Option,
) (Conn, error) {
	return DialTimeoutWithQueue(ioc, network, addr, 10*time.Second, q, opts...)
}

func DialTimeoutWithQueue(
	ioc *IO, network, addr string,
	timeout time.Duration, q *Queue,
	opts ...sonicopts.Option,
) (Conn, error) {
	fd, localAddr, remoteAddr, err := internal.ConnectTimeout(network, addr, timeout, opts...)
	if err != nil {
		return nil, err
	}

	return newQConn(ioc, fd, localAddr, remoteAddr, q), nil
}

func newQConn(
	ioc *IO,
	fd int,
	localAddr, remoteAddr net.Addr, q *Queue,
) *qConn {
	return &qConn{
		conn:  newConn(ioc, fd, localAddr, remoteAddr),
		queue: q,
	}
}
