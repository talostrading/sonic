package stream

import "github.com/talostrading/sonic/internal"

type readReactor struct {
	stream *Stream

	b        []byte
	callback func(error, int)
}

func newReadReactor(stream *Stream) *readReactor {
	return &readReactor{
		stream: stream,
	}
}

func (r *readReactor) prepare(b []byte, callback func(error, int)) error {
	r.b = b
	r.callback = callback
	r.stream.slot.Set(internal.ReadEvent, r.onReady)

	if err := r.stream.ioc.SetRead(r.stream.fd, &r.stream.slot); err != nil {
		return err
	}

	r.stream.ioc.RegisterRead(&r.stream.slot)

	return nil
}

func (r *readReactor) onReady(err error) {
	r.stream.ioc.DeregisterRead(&r.stream.slot)
	if err != nil {
		r.callback(err, 0)
	} else {
		r.stream.AsyncRead(r.b, r.callback)
	}
}
