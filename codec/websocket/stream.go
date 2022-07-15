package websocket

import (
	"github.com/talostrading/sonic"
)

var _ Stream = &WebsocketStream{}

type WebsocketStream struct {
	ioc     *sonic.IO
	stream  sonic.Stream
	src     *sonic.BytesBuffer // buffer for stream reads
	dst     *sonic.BytesBuffer // buffer for stream writes
	state   StreamState
	frame   *Frame // last read frame
	codec   sonic.Codec[*Frame]
	pending []*Frame
}

func NewWebsocketStream(ioc *sonic.IO) *WebsocketStream {
	s := &WebsocketStream{
		ioc:   ioc,
		src:   sonic.NewBytesBuffer(),
		dst:   sonic.NewBytesBuffer(),
		frame: NewFrame(),
		state: StateHandshake,
	}
	s.codec = NewFrameCodec(s.frame, s.src, s.dst)

	s.src.Prepare(4096)
	s.dst.Prepare(4096)

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
		if s.frame.IsFin() {
			return
		}
	}
}

func (s *WebsocketStream) ReadSome(b []byte) (n int, err error) {
	s.frame.payload = b
	for {
		fr, err := s.codec.Decode(s.src)

		if err != nil {
			return 0, err
		}

		if fr != nil {
			return s.frame.PayloadLen(), nil
		}

		if fr == nil {
			_, err = s.src.ReadFrom(s.stream)
			if err != nil {
				return 0, err
			}
		}
	}
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
		if s.frame.IsFin() {
			cb(err, readBytes)
		} else {
			s.asyncRead(b, readBytes, cb)
		}
	})
}

func (s *WebsocketStream) AsyncReadSome(b []byte, cb sonic.AsyncCallback) {
	n := 0

	fr, err := s.codec.Decode(s.src)
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
	s.src.AsyncReadFrom(s.stream, func(err error, n int) {
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
	return MessageType(s.frame.Opcode())
}
