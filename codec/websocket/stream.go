package websocket

import (
	"errors"
	"io"
	"net/url"
	"time"

	"github.com/talostrading/sonic"
	"github.com/talostrading/sonic/sonicerrors"
)

var (
	_           Stream = &WebsocketStream{}
	DialTimeout        = 5 * time.Second
)

type WebsocketStream struct {
	// Async operations executor.
	ioc *sonic.IO

	// Websocket role. Undefined until:
	// - AsyncAccept or Accept is called: the role is then set to RoleServer.
	// - AsyncHandshake or Accept is called: the role is then set to RoleServer.
	role Role

	// State of the stream.
	state StreamState

	// Buffer for stream reads.
	src *sonic.ByteBuffer

	// Buffer for stream writes.
	dst *sonic.ByteBuffer

	// Contains frames waiting to be sent to the peer.
	// Is emptied by AsyncFlush or Flush.
	pending []*Frame

	// Optional callback invoked when a control frame is received.
	ccb ControlCallback

	// The size of the currently read message.
	messageSize int

	// Handshake
	handshake *Handshake

	// FrameCodec used by codecConn to encode/decode frames
	codec *FrameCodec

	// Codec conn wrapping the underlying transport stream.
	codecConn sonic.CodecConn[*Frame, *Frame]
}

func NewWebsocketStream(ioc *sonic.IO) (s *WebsocketStream, err error) {
	s = &WebsocketStream{
		ioc: ioc,

		role:  RoleUndefined,
		src:   sonic.NewByteBuffer(),
		dst:   sonic.NewByteBuffer(),
		state: StateHandshake,
	}

	s.handshake, err = NewHandshake(s.src)
	if err != nil {
		return nil, err
	}

	s.codec, err = NewFrameCodec(s.src, s.dst)
	if err != nil {
		return nil, err
	}

	s.src.Reserve(4096)
	s.dst.Reserve(4096)

	return s, nil
}

func (s *WebsocketStream) reset() {
	s.role = RoleUndefined
	s.state = StateHandshake
	s.src.Reset()
	s.dst.Reset()
	s.handshake.Reset()
}

func (s *WebsocketStream) NextLayer() sonic.Conn {
	// TODO This next layer thing should be generic
	// after that we can do: return s.codecConn
	return s.codecConn.NextLayer()
}

func (s *WebsocketStream) SupportsUTF8() bool {
	return false
}

func (s *WebsocketStream) SupportsDeflate() bool {
	return false
}

func (s *WebsocketStream) canRead() bool {
	// we can only read from non-terminal states after a successful handshake
	return s.state == StateActive || s.state == StateClosedByUs
}

func (s *WebsocketStream) NextFrame() (f *Frame, err error) {
	err = s.Flush()

	if errors.Is(err, ErrMessageTooBig) {
		s.Close(CloseGoingAway, "payload too big")
		return nil, err
	}

	if err == nil && !s.canRead() {
		err = io.EOF
	}

	if err == nil {
		f, err = s.nextFrame()

		if err == io.EOF {
			s.state = StateTerminated
		}
	}

	return
}

func (s *WebsocketStream) nextFrame() (f *Frame, err error) {
	f, err = s.codecConn.ReadNext()
	if err == nil {
		err = s.handleFrame(f)
	}
	return
}

func (s *WebsocketStream) AsyncNextFrame(cb AsyncFrameHandler) {
	// TODO for later: here we flush first since we might need to reply
	// to ping/pong/close immediately, and only after that we try to
	// async read.
	//
	// I think we can just flush asynchronously while reading asynchronously at
	// the same time. I'm pretty sure this will work with a BlockingCodecStream.
	//
	// Not entirely sure about a NonblockingCodecStream.
	s.AsyncFlush(func(err error) {
		if errors.Is(err, ErrMessageTooBig) {
			s.AsyncClose(CloseGoingAway, "payload too big", func(err error) {})
			cb(ErrMessageTooBig, nil)
			return
		}

		if err == nil && !s.canRead() {
			err = io.EOF
		}

		if err == nil {
			s.asyncNextFrame(cb)
		} else {
			s.state = StateTerminated
			cb(err, nil)
		}
	})
}

func (s *WebsocketStream) asyncNextFrame(cb AsyncFrameHandler) {
	s.codecConn.AsyncReadNext(func(err error, f *Frame) {
		if err == nil {
			err = s.handleFrame(f)
		} else if err == io.EOF {
			s.state = StateTerminated
		}
		cb(err, f)
	})
}

func (s *WebsocketStream) NextMessage(b []byte) (mt MessageType, readBytes int, err error) {
	var (
		f            *Frame
		continuation = false
	)

	mt = TypeNone

	for {
		f, err = s.NextFrame()
		if err != nil {
			return mt, readBytes, err
		}

		if f.IsControl() {
			if s.ccb != nil {
				s.ccb(MessageType(f.Opcode()), f.payload)
			}
		} else {
			if mt == TypeNone {
				mt = MessageType(f.Opcode())
			}

			n := copy(b[readBytes:], f.Payload())
			readBytes += n

			if readBytes > MaxMessageSize || n != f.PayloadLen() {
				err = ErrMessageTooBig
				s.Close(CloseGoingAway, "payload too big")
				break
			}

			// verify continuation
			if !continuation {
				// this is the first frame of the series
				continuation = !f.IsFin()
				if f.IsContinuation() {
					err = ErrUnexpectedContinuation
				}
			} else {
				// we are past the first frame of the series
				continuation = !f.IsFin()
				if !f.IsContinuation() {
					err = ErrExpectedContinuation
				}
			}

			continuation = !f.IsFin()

			if err != nil || !continuation {
				break
			}
		}
	}

	return
}

func (s *WebsocketStream) AsyncNextMessage(b []byte, cb AsyncMessageHandler) {
	s.asyncNextMessage(b, 0, false, TypeNone, cb)
}

func (s *WebsocketStream) asyncNextMessage(
	b []byte,
	readBytes int,
	continuation bool,
	mt MessageType,
	cb AsyncMessageHandler,
) {
	s.AsyncNextFrame(func(err error, f *Frame) {
		if err != nil {
			cb(err, readBytes, mt)
		} else {
			if f.IsControl() {
				if s.ccb != nil {
					s.ccb(MessageType(f.Opcode()), f.payload)
				}

				s.asyncNextMessage(b, readBytes, continuation, mt, cb)
			} else {
				if mt == TypeNone {
					mt = MessageType(f.Opcode())
				}

				n := copy(b[readBytes:], f.Payload())
				readBytes += n

				if readBytes > MaxMessageSize || n != f.PayloadLen() {
					err = ErrMessageTooBig
					s.AsyncClose(CloseGoingAway, "payload too big", func(err error) {})
					cb(err, readBytes, mt)
					return
				}

				// verify continuation
				if !continuation {
					// this is the first frame of the series
					continuation = !f.IsFin()
					if f.IsContinuation() {
						err = ErrUnexpectedContinuation
					}
				} else {
					// we are past the first frame of the series
					continuation = !f.IsFin()
					if !f.IsContinuation() {
						err = ErrExpectedContinuation
					}
				}

				if err != nil || !continuation {
					cb(err, readBytes, mt)
				} else {
					s.asyncNextMessage(b, readBytes, continuation, mt, cb)
				}
			}
		}
	})
}

func (s *WebsocketStream) handleFrame(f *Frame) (err error) {
	err = s.verifyFrame(f)

	if err == nil {
		if f.IsControl() {
			err = s.handleControlFrame(f)
		} else {
			err = s.handleDataFrame(f)
		}
	}

	if err != nil {
		s.state = StateClosedByUs
		s.prepareClose(EncodeCloseFramePayload(CloseProtocolError, ""))
	}

	return err
}

func (s *WebsocketStream) verifyFrame(f *Frame) error {
	if f.IsRSV1() || f.IsRSV2() || f.IsRSV3() {
		return ErrNonZeroReservedBits
	}

	if s.role == RoleClient && f.IsMasked() {
		return ErrMaskedFramesFromServer
	}

	if s.role == RoleServer && !f.IsMasked() {
		return ErrUnmaskedFramesFromClient
	}

	return nil
}

func (s *WebsocketStream) handleControlFrame(f *Frame) (err error) {
	if !f.IsFin() {
		return ErrInvalidControlFrame
	}

	if f.PayloadLenType() > MaxControlFramePayloadSize {
		return ErrControlFrameTooBig
	}

	switch f.Opcode() {
	case OpcodePing:
		if s.state == StateActive {
			pongFrame := AcquireFrame()
			pongFrame.SetFin()
			pongFrame.SetPong()
			pongFrame.SetPayload(f.payload)
			if s.role == RoleClient {
				pongFrame.Mask()
			}
			s.pending = append(s.pending, pongFrame)
		}
	case OpcodePong:
	case OpcodeClose:
		switch s.state {
		case StateHandshake:
			panic("unreachable")
		case StateActive:
			s.state = StateClosedByPeer
			s.prepareClose(f.payload)
		case StateClosedByPeer, StateCloseAcked:
			// ignore
		case StateClosedByUs:
			// we received a reply from the peer
			s.state = StateCloseAcked
		case StateTerminated:
			panic("unreachable")
		}
	default:
		err = ErrInvalidControlFrame
	}

	return
}

func (s *WebsocketStream) handleDataFrame(f *Frame) error {
	if IsReserved(f.Opcode()) {
		return ErrReservedOpcode
	}
	return nil
}

func (s *WebsocketStream) Write(b []byte, mt MessageType) error {
	if len(b) > MaxMessageSize {
		return ErrMessageTooBig
	}

	if s.state == StateActive {
		f := AcquireFrame()
		f.SetFin()
		f.SetOpcode(Opcode(mt))
		f.SetPayload(b)

		s.prepareWrite(f)
		return s.Flush()
	}

	return sonicerrors.ErrCancelled
}

func (s *WebsocketStream) WriteFrame(f *Frame) error {
	if s.state == StateActive {
		s.prepareWrite(f)
		return s.Flush()
	} else {
		ReleaseFrame(f)
		return sonicerrors.ErrCancelled
	}
}

func (s *WebsocketStream) AsyncWrite(b []byte, mt MessageType, cb func(err error)) {
	if len(b) > MaxMessageSize {
		cb(ErrMessageTooBig)
		return
	}

	if s.state == StateActive {
		f := AcquireFrame()
		f.SetFin()
		f.SetOpcode(Opcode(mt))
		f.SetPayload(b)

		s.prepareWrite(f)
		s.AsyncFlush(cb)
	} else {
		cb(sonicerrors.ErrCancelled)
	}
}

func (s *WebsocketStream) AsyncWriteFrame(f *Frame, cb func(err error)) {
	if s.state == StateActive {
		s.prepareWrite(f)
		s.AsyncFlush(cb)
	} else {
		ReleaseFrame(f)
		cb(sonicerrors.ErrCancelled)
	}
}

func (s *WebsocketStream) prepareWrite(f *Frame) {
	switch s.role {
	case RoleClient:
		if !f.IsMasked() {
			f.Mask()
		}
	case RoleServer:
		if f.IsMasked() {
			f.Unmask()
		}
	}

	s.pending = append(s.pending, f)
}

func (s *WebsocketStream) AsyncClose(cc CloseCode, reason string, cb func(err error)) {
	switch s.state {
	case StateActive:
		s.state = StateClosedByUs
		s.prepareClose(EncodeCloseFramePayload(cc, reason))
		s.AsyncFlush(cb)
	case StateClosedByUs, StateHandshake:
		cb(sonicerrors.ErrCancelled)
	default:
		cb(io.EOF)
	}
}

func (s *WebsocketStream) Close(cc CloseCode, reason string) error {
	switch s.state {
	case StateActive:
		s.state = StateClosedByUs
		s.prepareClose(EncodeCloseFramePayload(cc, reason))
		return s.Flush()
	case StateClosedByUs, StateHandshake:
		return sonicerrors.ErrCancelled
	default:
		return io.EOF
	}
}

func (s *WebsocketStream) prepareClose(payload []byte) {
	closeFrame := AcquireFrame()
	closeFrame.SetFin()
	closeFrame.SetClose()
	closeFrame.SetPayload(payload)
	if s.role == RoleClient {
		closeFrame.Mask()
	}

	s.pending = append(s.pending, closeFrame)
}

func (s *WebsocketStream) Flush() (err error) {
	flushed := 0
	for i := 0; i < len(s.pending); i++ {
		err = s.codecConn.WriteNext(s.pending[i])
		if err != nil {
			break
		}
		ReleaseFrame(s.pending[i])
		flushed++
	}
	s.pending = s.pending[flushed:]

	return
}

func (s *WebsocketStream) AsyncFlush(cb func(err error)) {
	if len(s.pending) == 0 {
		cb(nil)
	} else {
		sent := s.pending[0]
		s.pending = s.pending[1:]

		s.codecConn.AsyncWriteNext(sent, func(err error) {
			ReleaseFrame(sent) // TODO can get rid of this and just allocate a single frame.

			if err != nil {
				cb(err)
			} else {
				s.AsyncFlush(cb)
			}
		})
	}
}

func (s *WebsocketStream) Pending() int {
	return len(s.pending)
}

func (s *WebsocketStream) State() StreamState {
	return s.state
}

func (s *WebsocketStream) Handshake(conn sonic.Conn, url *url.URL) (err error) {
	s.reset()

	s.role = RoleClient

	if err := s.handshake.Do(conn, url, RoleClient); err != nil {
		return err
	}

	err = s.prepare(conn)
	if err == nil {
		s.state = StateActive
	} else {
		s.state = StateTerminated
	}

	return err
}

func (s *WebsocketStream) AsyncHandshake(conn sonic.Conn, url *url.URL, cb func(err error)) {
	s.reset()

	s.role = RoleClient

	s.handshake.AsyncDo(conn, url, RoleClient, func(err error) {
		if err == nil {
			err = s.prepare(conn)
		}

		if err == nil {
			s.state = StateActive
		} else {
			s.state = StateTerminated
		}

		cb(err)
	})
}

func (s *WebsocketStream) prepare(conn sonic.Conn) (err error) {
	s.codecConn, err = sonic.NewCodecConn[*Frame, *Frame](s.ioc, conn, s.codec, s.src, s.dst)
	return err
}

func (s *WebsocketStream) Accept(conn sonic.Conn) error {
	s.reset()

	s.role = RoleServer

	panic("implement me")
}

func (s *WebsocketStream) AsyncAccept(conn sonic.Conn, cb func(error)) {
	s.reset()

	s.role = RoleServer

	panic("implement me")
}

func (s *WebsocketStream) SetControlCallback(ccb ControlCallback) {
	s.ccb = ccb
}

func (s *WebsocketStream) ControlCallback() ControlCallback {
	return s.ccb
}

func (s *WebsocketStream) SetMaxMessageSize(bytes int) {
	// This is just for checking against the length returned to the frame
	// header. The sizes of the buffers in which we read or write the messages
	// are dynamically adjusted in frame_codec.
	MaxMessageSize = bytes
}
