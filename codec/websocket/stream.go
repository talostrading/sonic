package websocket

import (
	"github.com/talostrading/sonic"
)

var _ Stream = &WebsocketStream{}

type WebsocketStream struct {
	ioc *sonic.IO

	stream sonic.Stream

	bi *sonic.BytesBuffer // buffer for stream reads
	bo *sonic.BytesBuffer // buffer for stream writes

	f *Frame // last read frame

	codecState *FrameCodecState
	codec      sonic.Codec[*Frame]

	state StreamState

	lt   MessageType // type of the last read message
	done bool        // true if we read a complete message from the stream

	pending []*Frame
}

func NewWebsocketStream(ioc *sonic.IO) *WebsocketStream {
	s := &WebsocketStream{
		ioc:   ioc,
		bi:    sonic.NewBytesBuffer(),
		bo:    sonic.NewBytesBuffer(),
		f:     NewFrame(),
		state: StateHandshake,
	}
	s.codecState = NewFrameCodecState(s.f)
	s.codec = NewFrameCodec(s.codecState)

	s.bi.Prepare(4096)
	s.bo.Prepare(4096)

	return s
}

func (s *WebsocketStream) NextLayer() sonic.Stream {
	return s.stream
}

func (s *WebsocketStream) DeflateSupported() bool {
	return false
}

func (s *WebsocketStream) Read(b []byte) (n int, err error) {
	var nn int

	for {
		nn, err = s.ReadSome(b)
		if err != nil {
			return
		}
		n += nn
		if s.done {
			return
		}
	}
}

func (s *WebsocketStream) ReadSome(b []byte) (n int, err error) {
	fr, err := s.codec.Decode(s.bi)
	if err == nil {
		if fr == nil {
			var nn int64
			nn, err = s.bi.ReadFrom(s.stream)
			n = int(nn)
		}
	}

	return
}

func (s *WebsocketStream) AsyncRead(b []byte, cb sonic.AsyncCallback) {
	s.asyncRead(b, 0, cb)
}

func (s *WebsocketStream) asyncRead(b []byte, readBytes int, cb sonic.AsyncCallback) {
	s.AsyncReadSome(b[readBytes:], func(err error, n int) {
		if err != nil {
			cb(err, readBytes)
			return
		}
		readBytes += n
		if s.done {
			cb(err, readBytes)
		} else {
			s.asyncRead(b, readBytes, cb)
		}
	})
}

func (s *WebsocketStream) AsyncReadSome(b []byte, cb sonic.AsyncCallback) {
	n := 0

	fr, err := s.codec.Decode(s.bi)
	if err == nil {
		if fr == nil {
			s.scheduleAsyncRead(b, cb)
		} else {
			n, err = s.update(fr)
		}
	}

	cb(err, n)
}

func (s *WebsocketStream) scheduleAsyncRead(b []byte, cb sonic.AsyncCallback) {
	s.bi.AsyncReadFrom(s.stream, func(err error, n int) {
		if err != nil {
			cb(err, n)
		} else {
			s.AsyncReadSome(b, cb)
		}
	})
}

func (s *WebsocketStream) WriteFrame(f *Frame) (err error) {
	return
}

func (s *WebsocketStream) AsyncWriteFrame(f *Frame, cb func(err error)) {

}

func (s *WebsocketStream) update(fr *Frame) (n int, err error) {
	return
}

func (s *WebsocketStream) Flush() error {
	return nil
}

func (s *WebsocketStream) AsyncFlush(cb func(err error)) {

}

func (s *WebsocketStream) Pending() int {
	return len(s.pending)
}

func (s *WebsocketStream) State() StreamState {
	return s.state
}

func (s *WebsocketStream) Handshake(addr string) error {
	return nil
}

func (s *WebsocketStream) AsyncHandshake(addr string, cb func(error)) {

}

func (s *WebsocketStream) Accept() error {
	return nil
}

func (s *WebsocketStream) AsyncAccept(func(error)) {

}

func (s *WebsocketStream) AsyncClose(cc CloseCode, reason string, cb func(err error)) {

}

func (s *WebsocketStream) Close(cc CloseCode, reason string) error {
	return nil
}

func (s *WebsocketStream) GotType() MessageType {
	return s.lt
}
