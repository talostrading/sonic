package sonicwebsocket

import (
	"bytes"
	"fmt"
	"io"
	"time"

	"github.com/talostrading/sonic"
)

func (s *WebsocketStream) Read(b []byte) (n int, err error) {
	if s.state == StateClosed {
		return 0, io.EOF
	}

	for {
		n, err = s.ReadSome(b)

		if s.IsMessageDone() {
			return
		}
	}
}

func (s *WebsocketStream) ReadSome(b []byte) (n int, err error) {
	if s.state == StateClosed {
		return 0, io.EOF
	}

	return s.readFrame(b)
}

func (s *WebsocketStream) readFrame(b []byte) (n int, err error) {
	s.rframe.Reset()

	if s.hasPendingReads() {
		return s.readPending(b)
	} else {
		nn, err := s.rframe.ReadFrom(s.stream)
		return int(nn), err
	}
}

func (s *WebsocketStream) AsyncRead(b []byte, cb sonic.AsyncCallback) {
	if s.state == StateClosed {
		cb(io.EOF, 0)
		return
	}

	s.asyncRead(b, 0, cb)
}

func (s *WebsocketStream) asyncRead(b []byte, readBytes int, cb sonic.AsyncCallback) {
	s.asyncReadFrame(b[readBytes:], func(err error, n int) {
		readBytes += n
		if err != nil {
			cb(err, readBytes)
		} else {
			s.onAsyncRead(b, readBytes, cb)
		}
	})
}

func (s *WebsocketStream) onAsyncRead(b []byte, readBytes int, cb sonic.AsyncCallback) {
	if s.IsMessageDone() {
		cb(nil, readBytes)
	} else if readBytes >= len(b) {
		cb(ErrPayloadTooBig, readBytes)
	} else {
		// continue reading frames to complete the current message
		s.asyncRead(b, readBytes, cb)
	}
}

func (s *WebsocketStream) AsyncReadSome(b []byte, cb sonic.AsyncCallback) {
	if s.state == StateClosed {
		cb(io.EOF, 0)
		return
	}

	s.asyncReadFrame(b, cb)
}

func (s *WebsocketStream) asyncReadFrame(b []byte, cb sonic.AsyncCallback) {
	s.rframe.Reset()

	if s.hasPendingReads() {
		n, err := s.readPending(b)
		if s.rframe.IsControl() {
			s.handleControlFrame(b, cb)
		} else {
			cb(err, n)
		}
	} else {
		s.asyncReadFrameHeader(b, cb)
	}
}

func (s *WebsocketStream) readPending(b []byte) (n int, err error) {
	_, err = s.rframe.ReadFrom(bytes.NewReader(s.rbuf))

	if err == nil {
		if len(b) < n {
			n = 0
			err = ErrPayloadTooBig
		} else {
			n = int(s.rframe.Len())
			s.rbuf = s.rbuf[:0]
			copy(b, s.rframe.Payload())
		}
	}

	return
}

func (s *WebsocketStream) hasPendingReads() bool {
	return len(s.rbuf) > 0
}

func (s *WebsocketStream) asyncReadFrameHeader(b []byte, cb sonic.AsyncCallback) {
	s.stream.AsyncReadAll(s.rframe.header[:2], func(err error, n int) {
		if err != nil {
			cb(ErrReadingHeader, 0)
		} else {
			if s.rframe.IsControl() {
				// Note that b and cb are meant to be used when handling data frames.
				// As such, we forward the arguments in all subsequent functions which handle
				// the control frame. The argument is transparent to all the subsequent functions.
				// After successful handling of the control frame, we reschedule an async read
				// with b and cb as arguments.
				s.asyncReadControlFrame(b, cb)
			} else {
				s.asyncReadDataFrame(b, cb)
			}
		}
	})
}

func (s *WebsocketStream) asyncReadControlFrame(bTransparent []byte, cbTransparent sonic.AsyncCallback) {
	// TODO all these panics should dissapear

	if !s.IsMessageDone() {
		panic(fmt.Errorf(
			"sonic-websocket: invalid control frame - FIN not set, control frame is fragmented frame=%s",
			s.rframe.String()))
	}

	m := s.rframe.readMore()
	if m > 0 {
		panic(fmt.Errorf(
			"sonic-websocket: invalid control frame - length is more than 125 bytes frame=%s",
			s.rframe.String()))
	} else {
		s.asyncReadPayload(s.controlFramePayload, func(err error, n int) {
			if err != nil {
				panic("sonic-websocket: could not read control frame payload")
			} else {
				s.controlFramePayload = s.controlFramePayload[:n]
				s.handleControlFrame(bTransparent, cbTransparent)
			}
		})
	}
}

func (s *WebsocketStream) handleControlFrame(bTransparent []byte, cbTransparent sonic.AsyncCallback) {
	switch s.rframe.Opcode() {
	case OpcodeClose:
		s.handleClose()
	case OpcodePing:
		s.handlePing()
	case OpcodePong:
		s.handlePong()
	}

	s.AsyncRead(bTransparent, cbTransparent)
}

func (s *WebsocketStream) handleClose() {
	switch s.state {
	case StateHandshake:
		// Not possible.
	case StateOpen:
		fmt.Println("receive control frme when open")
		// Received a close frame - MUST send a close frame back.
		// Note that there is no guarantee that any in-flight
		// writes will complete at this point.
		s.state = StateClosing

		closeFrame := AcquireFrame()
		closeFrame.SetPayload(s.rframe.Payload())
		closeFrame.SetOpcode(OpcodeClose)
		closeFrame.SetFin()
		if s.role == RoleClient {
			closeFrame.Mask()
		}
		fmt.Println("sneding close frame", closeFrame)

		s.asyncWriteFrame(closeFrame, func(err error, _ int) {
			if err != nil {
				panic(fmt.Errorf("sonic-websocket: could not send close reply frame err=%v", err))
			} else {
				s.state = StateClosed

				if s.role == RoleServer {
					s.stream.Close()
				} else {
					s.closeTimer.Arm(5*time.Second, func() {
						s.stream.Close()
						s.state = StateClosed
					})
				}
			}
			ReleaseFrame(closeFrame)
		})
	case StateClosing:
		// this happens when we called AsyncClose(...) or Close(...) before

		// TODO need a helper function or change handler signature to parse the reason for close which is in the first 2 bytes of the frame.
		if s.ccb != nil {
			s.ccb(OpcodeClose, s.rframe.Payload())
		}
		s.NextLayer().Close()
		s.state = StateClosed
		s.stream.Close()
	case StateClosed:
		// nothing
	default:
		panic(fmt.Errorf("sonic-websocket: unhandled state %s", s.state.String()))
	}
}

func (s *WebsocketStream) handlePing() {
	if s.state == StateOpen {
		pongFrame := AcquireFrame()
		pongFrame.SetOpcode(OpcodePong)
		pongFrame.SetFin()
		pongFrame.SetPayload(s.rframe.Payload())

		s.asyncWriteFrame(pongFrame, func(err error, n int) {
			if err != nil {
				// TODO retry timer?
				panic("sonic-websocket: could not send pong frame")
			}
			ReleaseFrame(pongFrame)
		})

		if s.ccb != nil {
			s.ccb(OpcodePing, s.rframe.Payload())
		}
	}
}

func (s *WebsocketStream) handlePong() {
	if s.state == StateOpen {
		if s.ccb != nil {
			s.ccb(OpcodePong, s.rframe.Payload())
		}
	}
}

func (s *WebsocketStream) asyncReadDataFrame(b []byte, cb sonic.AsyncCallback) {
	m := s.rframe.readMore()
	if m > 0 {
		s.asyncReadFrameExtraLength(m, b, cb)
	} else {
		if s.rframe.IsMasked() {
			s.asyncReadFrameMask(b, cb)
		} else {
			s.asyncReadPayload(b, cb)
		}
	}
}

func (s *WebsocketStream) asyncReadFrameExtraLength(m int, b []byte, cb sonic.AsyncCallback) {
	s.stream.AsyncReadAll(s.rframe.header[2:m+2], func(err error, n int) {
		if err != nil {
			cb(ErrReadingExtendedLength, 0)
		} else {
			if s.rframe.Len() > s.rmax {
				cb(ErrPayloadTooBig, 0)
			} else {
				if s.rframe.IsMasked() {
					s.asyncReadFrameMask(b, cb)
				} else {
					s.asyncReadPayload(b, cb)
				}
			}
		}
	})
}

func (s *WebsocketStream) asyncReadFrameMask(b []byte, cb sonic.AsyncCallback) {
	s.stream.AsyncReadAll(s.rframe.mask[:4], func(err error, n int) {
		if err != nil {
			cb(ErrReadingMask, 0)
		} else {
			s.asyncReadPayload(b, cb)
		}
	})
}

func (s *WebsocketStream) asyncReadPayload(b []byte, cb sonic.AsyncCallback) {
	if pl := s.rframe.Len(); pl > 0 {
		payloadLen := int64(pl)
		if remaining := payloadLen - int64(cap(b)); remaining > 0 {
			cb(ErrPayloadTooBig, 0)
		} else {
			s.stream.AsyncReadAll(b[:payloadLen], func(err error, n int) {
				if err != nil {
					cb(err, n)
				} else {
					cb(nil, n)
				}
			})
		}
	}
}
