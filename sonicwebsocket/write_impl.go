package sonicwebsocket

import (
	"fmt"
	"io"

	"github.com/talostrading/sonic"
)

func (s *WebsocketStream) Write(b []byte) (n int, err error) {
	if s.state != StateOpen {
		return 0, io.EOF
	}

	fr := s.makeFrame(true, b)
	nn, err := fr.WriteTo(s.stream)
	ReleaseFrame(fr)
	return int(nn), err
}

// WriteSome writes some message data.
func (s *WebsocketStream) WriteSome(fin bool, b []byte) (n int, err error) {
	if s.state != StateOpen {
		return 0, io.EOF
	}

	fr := s.makeFrame(fin, b)
	nn, err := fr.WriteTo(s.stream)
	ReleaseFrame(fr)
	return int(nn), err
}

// AsyncWrite writes a complete message asynchronously.
func (s *WebsocketStream) AsyncWrite(b []byte, cb sonic.AsyncCallback) {
	if s.state != StateOpen {
		cb(io.EOF, 0)
		return
	}

	fr := s.makeFrame(true, b)

	s.asyncWriteFrame(fr, func(err error, n int) {
		cb(err, n)
		ReleaseFrame(fr)
	})
}

// AsyncWriteSome writes some message data asynchronously.
func (s *WebsocketStream) AsyncWriteSome(fin bool, b []byte, cb sonic.AsyncCallback) {
	if s.state != StateOpen {
		cb(io.EOF, 0)
		return
	}

	fr := s.makeFrame(fin, b)

	s.asyncWriteFrame(fr, func(err error, n int) {
		cb(err, n)
		ReleaseFrame(fr)
	})
}

// TODO get rid of makeFrame, can just reuse
func (s *WebsocketStream) makeFrame(fin bool, payload []byte) *frame {
	fr := AcquireFrame()

	if fin {
		fr.SetFin()
	}

	if s.wopcode == OpcodeText {
		fr.SetText()
	} else if s.wopcode == OpcodeBinary {
		fr.SetBinary()
	} else {
		panic(fmt.Errorf("invalid write_op opcode=%s", s.wopcode))
	}

	fr.SetPayload(payload)

	if s.role == RoleClient {
		fr.Mask()
	}

	return fr
}

func (s *WebsocketStream) asyncWriteFrame(fr *frame, cb sonic.AsyncCallback) {
	s.wbuf.Reset()

	nn, err := fr.WriteTo(s.wbuf)
	n := int(nn)
	if err != nil {
		cb(err, n)
	} else {
		b := s.wbuf.Bytes()
		b = b[:n]
		s.stream.AsyncWriteAll(b, cb)
	}
}
