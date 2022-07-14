package websocket

import (
	"github.com/talostrading/sonic"
)

type WebsocketStream struct {
	ioc   *sonic.IO
	buf   *sonic.BytesBuffer
	codec *FrameCodec

	bi *sonic.BytesBuffer
	bo *sonic.BytesBuffer

	stream sonic.Stream
}

func NewWebsocketStream(ioc *sonic.IO) *WebsocketStream {
	s := &WebsocketStream{
		ioc:   ioc,
		buf:   sonic.NewBytesBufferWithCapacity(4096),
		codec: NewFrameCodec(),
	}
	return s
}

func (s *WebsocketStream) AsyncRead(b []byte, cb sonic.AsyncCallback) {
	s.AsyncReadSome(b, func(err error, n int) {
		if err == nil && !s.IsMessageDone() {
			s.AsyncRead(b[n:], cb)
		} else {
			cb(err, n)
		}
	})
}

func (s *WebsocketStream) AsyncReadSome(b []byte, cb sonic.AsyncCallback) {
	s.asyncReadNow(b, cb)
}

func (s *WebsocketStream) asyncReadNow(b []byte, cb sonic.AsyncCallback) {
	n := 0

	fr, err := s.codec.Decode(s.bi)
	if err == nil {
		if fr == nil {
			s.scheduleAsyncRead(b, cb)
		} else {
			n, err = s.update(b, fr)
		}
	}

	cb(err, n)
}

func (s *WebsocketStream) scheduleAsyncRead(b []byte, cb sonic.AsyncCallback) {
	s.bi.AsyncReadFrom(s.stream, func(err error, n int) {
		if err != nil {
			cb(err, n)
		} else {
			s.asyncReadNow(b, cb)
		}
	})
}

func (s *WebsocketStream) update(b []byte, fr *Frame) (n int, err error) {
	n = copy(b, fr.payload)
	return
}

func (s *WebsocketStream) IsMessageDone() bool {
	return true
}
