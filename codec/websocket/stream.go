package websocket

/*
gosec G505 G401:
The WebSocket protocol mandates the use of sha1 hashes in the
opening handshake initiated by the client. Sha1 is used purely
as a hashing function. This hash not used to provide any security.
It is simply used as a verification step by the protocol to ensure
that the server speaks the WebSocket protocol.
This verification is needed as the handshake is done over HTTP -
without it any http server might accept the websocket handshake, at
which point the protocol will be violated on subsequent read/writes,
when the client cannot parse what the server sends.
*/

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"crypto/sha1" //#nosec G505
	"crypto/tls"
	"encoding/base64"
	"errors"
	"fmt"
	"hash"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync"
	"syscall"
	"unicode/utf8"

	"github.com/talostrading/sonic"
	"github.com/talostrading/sonic/sonicerrors"
	"github.com/talostrading/sonic/sonicopts"
)

type Stream struct {
	ioc *sonic.IO

	// User provided TLS config; nil if we don't use TLS
	tls *tls.Config

	// Underlying transport stream that we async adapt from the net.Conn.
	stream sonic.Stream
	conn   net.Conn

	// Codec stream wrapping the underlying transport stream.
	codec     *FrameCodec
	codecConn *sonic.CodecConn[Frame, Frame]

	// Websocket role: client or server.
	role Role

	// State of the stream.
	state StreamState

	// Hashes the Sec-Websocket-Key when the stream is a client.
	hasher hash.Hash

	// Buffer for stream reads.
	src *sonic.ByteBuffer

	// Buffer for stream writes.
	dst *sonic.ByteBuffer

	// Contains the handshake response. Is emptied after the handshake is over.
	handshakeBuffer []byte

	// Contains frames waiting to be sent to the peer. Is emptied by AsyncFlush or Flush.
	pendingFrames []*Frame

	// Optional callback invoked when a control frame is received.
	controlCallback ControlCallback

	// Optional callback invoked when an upgrade request is sent.
	upgradeRequestCallback UpgradeRequestCallback

	// Optional callback invoked when an upgrade response is received.
	upgradeResponseCallback UpgradeResponseCallback

	// Used to establish a TCP connection to the peer with a timeout.
	dialer *net.Dialer

	framePool sync.Pool

	maxMessageSize int

	validateUTF8 bool
}

func NewWebsocketStream(ioc *sonic.IO, tls *tls.Config, role Role) (s *Stream, err error) {
	s = &Stream{
		ioc:   ioc,
		tls:   tls,
		role:  role,
		src:   sonic.NewByteBuffer(),
		dst:   sonic.NewByteBuffer(),
		state: StateHandshake,
		/* #nosec G401 */
		hasher:          sha1.New(),
		handshakeBuffer: make([]byte, 1024),
		dialer: &net.Dialer{
			Timeout: DialTimeout,
		},
		framePool: sync.Pool{
			New: func() interface{} {
				frame := NewFrame()
				return &frame
			},
		},
		maxMessageSize: DefaultMaxMessageSize,
		validateUTF8:   false,
	}

	s.src.Reserve(4096)
	s.dst.Reserve(4096)

	return s, nil
}

// The recommended way to create a `Frame`. This function takes care to allocate enough bytes to encode the WebSocket
// header and apply client side masking if the `Stream`'s role is `RoleClient`.
func (s *Stream) AcquireFrame() *Frame {
	f := s.framePool.Get().(*Frame)
	if s.role == RoleClient {
		// This just reserves the 4 bytes needed for the mask in order to encode the payload correctly, since it follows
		// the mask in byte order. The actual mask is set after `f.SetPayload` in `prepareWrite`.
		f.SetIsMasked()
	}
	return f
}

func (s *Stream) releaseFrame(f *Frame) {
	f.Reset()
	s.framePool.Put(f)
}

// This stream is either a client or a server. The main differences are in how the opening/closing handshake is done and
// in the fact that payloads sent by the client to the server are masked.
func (s *Stream) Role() Role {
	return s.role
}

// init is run when we transition into StateActive which happens
// after a successful handshake.
func (s *Stream) init(stream sonic.Stream) (err error) {
	if s.state != StateActive {
		return fmt.Errorf("stream must be in StateActive")
	}

	s.stream = stream
	s.codec = NewFrameCodec(s.src, s.dst, s.maxMessageSize)
	s.codecConn, err = sonic.NewCodecConn[Frame, Frame](stream, s.codec, s.src, s.dst)
	return
}

func (s *Stream) reset() {
	s.handshakeBuffer = s.handshakeBuffer[:cap(s.handshakeBuffer)]
	s.state = StateHandshake
	s.stream = nil
	s.conn = nil
	s.src.Reset()
	s.dst.Reset()
}

// Returns the stream through which IO is done.
func (s *Stream) NextLayer() sonic.Stream {
	if s.codecConn != nil {
		return s.codecConn.NextLayer()
	}
	return nil
}

// SupportsUTF8 indicates that the stream can optionally perform UTF-8 validation on the payloads of Text frames.
// Validation is disabled by default and can be toggled with `ValidateUTF8(bool)`.
func (s *Stream) SupportsUTF8() bool {
	return true
}

// ValidatesUTF8 indicates if UTF-8 validation is performed on the payloads of Text frames. Validation is disabled by
// default and can be toggled with `ValidateUTF8(bool)`.
func (s *Stream) ValidatesUTF8() bool {
	return s.validateUTF8
}

// ValidateUTF8 toggles UTF8 validation done on the payloads of Text frames.
func (s *Stream) ValidateUTF8(v bool) *Stream {
	s.validateUTF8 = v
	return s
}

func (s *Stream) SupportsDeflate() bool {
	return false
}

func (s *Stream) canRead() bool {
	// we can only read from non-terminal states after a successful handshake
	return s.state == StateActive || s.state == StateClosedByUs
}

// NextFrame reads and returns the next frame.
//
// This call first flushes any pending control frames to the underlying stream.
//
// This call blocks until one of the following conditions is true:
//   - an error occurs while flushing the pending control frames
//   - an error occurs when reading/decoding the message bytes from the underlying stream
//   - a frame is successfully read from the underlying stream
func (s *Stream) NextFrame() (f Frame, err error) {
	err = s.Flush()

	if errors.Is(err, ErrMessageTooBig) {
		_ = s.Close(CloseGoingAway, "payload too big")
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

func (s *Stream) nextFrame() (f Frame, err error) {
	f, err = s.codecConn.ReadNext()

	// If we get an EOF error, TCP stream was closed
	// This is an abnormal closure from the server
	if s.state != StateTerminated && err == io.EOF {
		s.state = StateTerminated

		// Prepare and return the 1006 close frame directly to client
		f = NewFrame()
		f.SetFIN().SetClose().SetPayload(EncodeCloseFramePayload(CloseAbnormal, ""))
	}

	if err == nil {
		err = s.handleFrame(f)
	}
	return
}

// AsyncNextFrame reads and returns the next frame asynchronously.
//
// This call first flushes any pending control frames to the underlying stream asynchronously.
//
// This call does not block. The provided callback is invoked when one of the following happens:
//   - an error occurs while flushing the pending control frames
//   - an error occurs when reading/decoding the message bytes from the underlying stream
//   - a frame is successfully read from the underlying stream
func (s *Stream) AsyncNextFrame(callback AsyncFrameCallback) {
	s.AsyncFlush(func(err error) {
		if errors.Is(err, ErrMessageTooBig) {
			s.AsyncClose(CloseGoingAway, "payload too big", func(err error) {})
			callback(ErrMessageTooBig, nil)
			return
		}

		if err == nil && !s.canRead() {
			err = io.EOF
		}

		if err == nil {
			s.asyncNextFrame(callback)
		} else {
			s.state = StateTerminated
			callback(err, nil)
		}
	})
}

func (s *Stream) asyncNextFrame(callback AsyncFrameCallback) {
	s.codecConn.AsyncReadNext(func(err error, f Frame) {
		if err == nil {
			err = s.handleFrame(f)

			// If we get an EOF error, TCP stream was closed
			// This is an abnormal closure from the server
		} else if s.state != StateTerminated && err == io.EOF {
			s.state = StateTerminated

			// Prepare and return the 1006 close frame directly
			f = NewFrame()
			f.SetFIN().SetClose().SetPayload(EncodeCloseFramePayload(CloseAbnormal, ""))
		}

		callback(err, f)
	})
}

// NextMessage reads the payload of the next message into the supplied buffer. Message fragmentation is automatically
// handled by the implementation.
//
// This call first flushes any pending control frames to the underlying stream.
//
// This call blocks until one of the following conditions is true:
//   - an error occurs while flushing the pending control frames
//   - an error occurs when reading/decoding the message from the underlying stream
//   - the payload of the message is successfully read into the supplied buffer, after all message fragments are read
func (s *Stream) NextMessage(b []byte) (messageType MessageType, readBytes int, err error) {
	var (
		f            Frame
		continuation = false
	)
	messageType = TypeNone

	for {
		f, err = s.NextFrame()
		if err != nil {
			return messageType, readBytes, err
		}

		if f.Opcode().IsControl() {
			if s.controlCallback != nil {
				s.controlCallback(MessageType(f.Opcode()), f.Payload())
			}
		} else {
			if messageType == TypeNone {
				messageType = MessageType(f.Opcode())
			}

			n := copy(b[readBytes:], f.Payload())
			readBytes += n

			if readBytes > s.maxMessageSize || n != f.PayloadLength() {
				err = ErrMessageTooBig
				_ = s.Close(CloseGoingAway, "payload too big")
				break
			}

			// verify continuation
			if !continuation {
				// this is the first frame of the series
				continuation = !f.IsFIN() //nolint:ineffassign
				if f.Opcode().IsContinuation() {
					err = ErrUnexpectedContinuation
				}
			} else {
				// we are past the first frame of the series
				continuation = !f.IsFIN() //nolint:ineffassign
				if !f.Opcode().IsContinuation() {
					err = ErrExpectedContinuation
				}
			}

			continuation = !f.IsFIN()

			if err != nil || !continuation {
				break
			}
		}
	}

	return
}

// AsyncNextMessage reads the payload of the next message into the supplied buffer asynchronously. Message fragmentation
// is automatically handled by the implementation.
//
// This call first flushes any pending control frames to the underlying stream asynchronously.
//
// This call does not block. The provided callback is invoked when one of the following happens:
//   - an error occurs while flushing the pending control frames
//   - an error occurs when reading/decoding the message bytes from the underlying stream
//   - the payload of the message is successfully read into the supplied buffer, after all message fragments are read
func (s *Stream) AsyncNextMessage(b []byte, callback AsyncMessageCallback) {
	s.asyncNextMessage(b, 0, false, TypeNone, callback)
}

func (s *Stream) asyncNextMessage(
	b []byte,
	readBytes int,
	continuation bool,
	messageType MessageType,
	callback AsyncMessageCallback,
) {
	s.AsyncNextFrame(func(err error, f Frame) {
		if err != nil {
			callback(err, readBytes, messageType)
		} else {
			if f.Opcode().IsControl() {
				if s.controlCallback != nil {
					s.controlCallback(MessageType(f.Opcode()), f.Payload())
				}

				s.asyncNextMessage(b, readBytes, continuation, messageType, callback)
			} else {
				if messageType == TypeNone {
					messageType = MessageType(f.Opcode())
				}

				n := copy(b[readBytes:], f.Payload())
				readBytes += n

				if readBytes > s.maxMessageSize || n != f.PayloadLength() {
					err = ErrMessageTooBig
					s.AsyncClose(
						CloseGoingAway,
						"payload too big",
						func(err error) {},
					)
					callback(err, readBytes, messageType)
					return
				}

				// verify continuation
				if !continuation {
					// this is the first frame of the series
					continuation = !f.IsFIN()
					if f.Opcode().IsContinuation() {
						err = ErrUnexpectedContinuation
					}
				} else {
					// we are past the first frame of the series
					continuation = !f.IsFIN()
					if !f.Opcode().IsContinuation() {
						err = ErrExpectedContinuation
					}
				}

				if err != nil || !continuation {
					callback(err, readBytes, messageType)
				} else {
					s.asyncNextMessage(b, readBytes, continuation, messageType, callback)
				}
			}
		}
	})
}

func (s *Stream) handleFrame(f Frame) (err error) {
	err = s.verifyFrame(f)

	if err == nil {
		if f.Opcode().IsControl() {
			err = s.handleControlFrame(f)
		} else {
			err = s.handleDataFrame(f)
		}
	}

	if err != nil {
		s.state = StateClosedByUs
		// TODO consider flushing the close
		s.prepareClose(EncodeCloseFramePayload(CloseProtocolError, ""))
	}

	return err
}

func (s *Stream) verifyFrame(f Frame) error {
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

func (s *Stream) handleControlFrame(f Frame) (err error) {
	if !f.IsFIN() {
		return ErrInvalidControlFrame
	}

	if f.PayloadLength() > MaxControlFramePayloadLength {
		return ErrControlFrameTooBig
	}

	switch f.Opcode() {
	case OpcodePing:
		if s.state == StateActive {
			pongFrame := s.AcquireFrame().
				SetFIN().
				SetPong().
				SetPayload(f.Payload())
			s.prepareWrite(pongFrame)
		}
	case OpcodePong:
	case OpcodeClose:
		switch s.state {
		case StateHandshake:
			panic("unreachable")
		case StateActive:
			s.state = StateClosedByPeer

			payload := f.Payload()
			// The first 2 bytes are the close code. The rest, if any, is the reason - this must be UTF-8.
			if len(payload) >= 2 {
				if !utf8.Valid(payload[2:]) {
					s.prepareClose(EncodeCloseCode(CloseProtocolError))
				} else {
					closeCode := DecodeCloseCode(payload)
					if !ValidCloseCode(closeCode) {
						s.prepareClose(EncodeCloseCode(CloseProtocolError))
					} else {
						s.prepareClose(f.Payload())
					}
				}
			} else if len(payload) > 0 {
				s.prepareClose(EncodeCloseCode(CloseProtocolError))
			} else {
				s.prepareClose(EncodeCloseCode(CloseNormal))
			}
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

func (s *Stream) handleDataFrame(f Frame) error {
	if f.Opcode().IsReserved() {
		return ErrReservedOpcode
	}

	if f.Opcode().IsText() && s.validateUTF8 {
		if !utf8.Valid(f.Payload()) {
			return ErrInvalidUTF8
		}
	}

	return nil
}

// Write writes the supplied buffer as a single message with the given type to the underlying stream.
//
// This call first flushes any pending control frames to the underlying stream.
//
// This call blocks until one of the following conditions is true:
//   - an error occurs while flushing the pending control frames
//   - an error occurs during the write
//   - the message is successfully written to the underlying stream
func (s *Stream) Write(b []byte, messageType MessageType) error {
	if len(b) > s.maxMessageSize {
		return ErrMessageTooBig
	}

	if s.state == StateActive {
		// reserve space for mask if client
		f := s.AcquireFrame().
			SetFIN().
			SetOpcode(Opcode(messageType)).
			SetPayload(b)
		s.prepareWrite(f)
		return s.Flush()
	}

	return sonicerrors.ErrCancelled
}

// WriteFrame writes the supplied frame to the underlying stream.
//
// This call first flushes any pending control frames to the underlying stream.
//
// This call blocks until one of the following conditions is true:
//   - an error occurs while flushing the pending control frames
//   - an error occurs during the write
//   - the frame is successfully written to the underlying stream
func (s *Stream) WriteFrame(f *Frame) error {
	if s.state == StateActive {
		s.prepareWrite(f)
		return s.Flush()
	} else {
		s.releaseFrame(f)
		return sonicerrors.ErrCancelled
	}
}

// AsyncWrite writes the supplied buffer as a single message with the given type to the underlying stream
// asynchronously.
//
// This call first flushes any pending control frames to the underlying stream asynchronously.
//
// This call does not block. The provided callback is invoked when one of the following happens:
//   - an error occurs while flushing the pending control frames
//   - an error occurs during the write
//   - the message is successfully written to the underlying stream
func (s *Stream) AsyncWrite(b []byte, messageType MessageType, callback func(err error)) {
	if len(b) > s.maxMessageSize {
		callback(ErrMessageTooBig)
		return
	}

	if s.state == StateActive {
		f := s.AcquireFrame().
			SetFIN().
			SetOpcode(Opcode(messageType)).
			SetPayload(b)
		s.prepareWrite(f)
		s.AsyncFlush(callback)
	} else {
		callback(sonicerrors.ErrCancelled)
	}
}

// AsyncWriteFrame writes the supplied frame to the underlying stream asynchronously.
//
// This call first flushes any pending control frames to the underlying stream asynchronously.
//
// This call does not block. The provided callback is invoked when one of the following happens:
//   - an error occurs while flushing the pending control frames
//   - an error occurs during the write
//   - the frame is successfully written to the underlying stream
func (s *Stream) AsyncWriteFrame(f *Frame, callback func(err error)) {
	if s.state == StateActive {
		s.prepareWrite(f)
		s.AsyncFlush(callback)
	} else {
		s.releaseFrame(f)
		callback(sonicerrors.ErrCancelled)
	}
}

func (s *Stream) prepareWrite(f *Frame) {
	if s.role == RoleClient {
		f.MaskPayload()
	}
	s.pendingFrames = append(s.pendingFrames, f)
}

// AsyncClose sends a websocket close control frame asynchronously.
//
// This function is used to send a close frame which begins the WebSocket closing handshake. The session ends when both
// ends of the connection have sent and received a close frame.
//
// The callback is called if one of the following conditions is true:
//   - the close frame is written
//   - an error occurs
//
// After beginning the closing handshake, the program should not write further messages, pings, pongs or close
// frames. Instead, the program should continue reading messages until the closing handshake is complete or an error
// occurs.
func (s *Stream) AsyncClose(closeCode CloseCode, reason string, callback func(err error)) {
	switch s.state {
	case StateActive:
		s.state = StateClosedByUs
		s.prepareClose(EncodeCloseFramePayload(closeCode, reason))
		s.AsyncFlush(callback)
	case StateClosedByUs, StateHandshake:
		callback(sonicerrors.ErrCancelled)
	default:
		callback(io.EOF)
	}
}

// Close sends a websocket close control frame asynchronously.
//
// This function is used to send a close frame which begins the WebSocket closing handshake. The session ends when both
// ends of the connection have sent and received a close frame.
//
// The call blocks until one of the following conditions is true:
//   - the close frame is written
//   - an error occurs
//
// After beginning the closing handshake, the program should not write further messages, pings, pongs or close
// frames. Instead, the program should continue reading messages until the closing handshake is complete or an error
// occurs.
func (s *Stream) Close(cc CloseCode, reason string) error {
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

func (s *Stream) prepareClose(payload []byte) {
	closeFrame := s.AcquireFrame().
		SetFIN().
		SetClose().
		SetPayload(payload)
	s.prepareWrite(closeFrame)
}

// Flush writes any pending control frames to the underlying stream.
//
// This call blocks.
func (s *Stream) Flush() (err error) {
	flushed := 0
	for i := 0; i < len(s.pendingFrames); i++ {
		_, err = s.codecConn.WriteNext(*s.pendingFrames[i])
		if err != nil {
			break
		}
		s.releaseFrame(s.pendingFrames[i])
		flushed++
	}
	s.pendingFrames = s.pendingFrames[flushed:]

	return
}

// Flush writes any pending control frames to the underlying stream asynchronously.
//
// This call does not block.
func (s *Stream) AsyncFlush(callback func(err error)) {
	if len(s.pendingFrames) == 0 {
		callback(nil)
	} else {
		sent := s.pendingFrames[0]
		s.pendingFrames = s.pendingFrames[1:]

		s.codecConn.AsyncWriteNext(*sent, func(err error, _ int) {
			s.releaseFrame(sent)

			if err != nil {
				callback(err)
			} else {
				s.AsyncFlush(callback)
			}
		})
	}
}

// Pending returns the number of currently pending control frames waiting to be flushed.
func (s *Stream) Pending() int {
	return len(s.pendingFrames)
}

func (s *Stream) State() StreamState {
	return s.state
}

// Handshake performs the client handshake. This call blocks.
//
// The call blocks until one of the following conditions is true:
//   - the HTTP1.1 request is sent and the response is received
//   - an error occurs
//
// Extra headers should be generated by calling `ExtraHeader(...)`.
func (s *Stream) Handshake(addr string, extraHeaders ...Header) (err error) {
	if s.role != RoleClient {
		return ErrWrongHandshakeRole
	}

	s.reset()

	var stream sonic.Stream

	done := make(chan struct{}, 1)
	s.handshake(addr, extraHeaders, func(rerr error, rstream sonic.Stream) {
		err = rerr
		stream = rstream
		done <- struct{}{}
	})
	<-done

	if err != nil {
		s.state = StateTerminated
	} else {
		s.state = StateActive
		err = s.init(stream)
	}

	return
}

// AsyncHandshake performs the WebSocket handshake asynchronously in the client role.
//
// This call does not block. The provided callback is called when the request is sent and the response is
// received or when an error occurs.
//
// Extra headers should be generated by calling `ExtraHeader(...)`.
func (s *Stream) AsyncHandshake(addr string, callback func(error), extraHeaders ...Header) {
	if s.role != RoleClient {
		callback(ErrWrongHandshakeRole)
		return
	}

	s.reset()

	// I know, this is horrible, but if you help me write a TLS client for sonic
	// we can asynchronously dial endpoints and remove the need for a goroutine
	go func() {
		s.handshake(addr, extraHeaders, func(err error, stream sonic.Stream) {
			// TODO maybe report this error somehow although this is very fatal
			_ = s.ioc.Post(func() {
				if err != nil {
					s.state = StateTerminated
				} else {
					s.state = StateActive
					err = s.init(stream)
				}
				callback(err)
			})
		})
	}()
}

func (s *Stream) handshake(addr string, headers []Header, callback func(err error, stream sonic.Stream)) {
	url, err := s.resolve(addr)
	if err != nil {
		callback(err, nil)
	} else {
		s.dial(url, func(err error, stream sonic.Stream) {
			if err == nil {
				err = s.upgrade(url, stream, headers)
			}
			callback(err, stream)
		})
	}
}

func (s *Stream) resolve(addr string) (resolvedUrl *url.URL, err error) {
	resolvedUrl, err = url.Parse(addr)
	if err == nil {
		switch resolvedUrl.Scheme {
		case "ws":
			resolvedUrl.Scheme = "http"
		case "wss":
			resolvedUrl.Scheme = "https"
		default:
			err = ErrInvalidAddress
		}
	}

	return
}

func (s *Stream) dial(url *url.URL, callback func(err error, stream sonic.Stream)) {
	var (
		err  error
		sc   syscall.Conn
		port = url.Port()
	)

	switch url.Scheme {
	case "http":
		if port == "" {
			port = "80"
		}
		addr := url.Hostname() + ":" + port
		s.conn, err = net.DialTimeout("tcp", addr, DialTimeout)
		if err == nil {
			sc = s.conn.(syscall.Conn)
		} else {
			// This is needed otherwise the net.Conn interface will be pointing
			// to a nil pointer. Calling something like CloseNextLayer will
			// produce a panic then.
			s.conn = nil
		}
	case "https":
		if s.tls == nil {
			err = fmt.Errorf(
				"wss:// scheme endpoints require a TLS configuration",
			)
		}

		if err == nil {
			if port == "" {
				port = "443"
			}
			addr := url.Hostname() + ":" + port
			s.conn, err = tls.DialWithDialer(s.dialer, "tcp", addr, s.tls)
			if err == nil {
				sc = s.conn.(*tls.Conn).NetConn().(syscall.Conn)
			} else {
				// This is needed otherwise the net.Conn interface will be
				// pointing to a nil pointer. Calling something like
				// CloseNextLayer will produce a panic then.
				s.conn = nil
			}
		}
	default:
		err = fmt.Errorf("invalid url scheme=%s", url.Scheme)
	}

	if err == nil {
		// s.ioc is not used by this constructor, so there is NO a race
		// condition on the io context.
		sonic.NewAsyncAdapter(
			s.ioc, sc, s.conn, func(err error, stream *sonic.AsyncAdapter) {
				callback(err, stream)
			}, sonicopts.NoDelay(true))
	} else {
		callback(err, nil)
	}
}

func (s *Stream) upgrade(uri *url.URL, stream sonic.Stream, headers []Header) error {
	req, err := http.NewRequest("GET", uri.String(), nil)
	if err != nil {
		return err
	}

	sentKey, expectedKey := s.makeHandshakeKey()
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Connection", "upgrade")
	req.Header.Set("Sec-WebSocket-Key", string(sentKey))
	req.Header.Set("Sec-Websocket-Version", "13")

	for _, header := range headers {
		if header.CanonicalKey {
			req.Header.Del(header.Key)
			for _, value := range header.Values {
				req.Header.Add(header.Key, value)
			}
		} else {
			delete(req.Header, header.Key)
			req.Header[header.Key] = append(
				req.Header[header.Key],
				header.Values...,
			)
		}
	}

	if s.upgradeRequestCallback != nil {
		s.upgradeRequestCallback(req)
	}

	err = req.Write(stream)
	if err != nil {
		return err
	}

	s.handshakeBuffer = s.handshakeBuffer[:cap(s.handshakeBuffer)]
	n, err := stream.Read(s.handshakeBuffer)
	if err != nil {
		return err
	}
	s.handshakeBuffer = s.handshakeBuffer[:n]
	rd := bytes.NewReader(s.handshakeBuffer)
	res, err := http.ReadResponse(bufio.NewReader(rd), req)
	if err != nil {
		return err
	}

	rawRes, err := httputil.DumpResponse(res, true)
	if err != nil {
		return err
	}

	resLen := len(rawRes)
	extra := len(s.handshakeBuffer) - resLen
	if extra > 0 {
		// we got some frames as well with the handshake so we can put
		// them in src for later decoding before clearing the handshake
		// buffer
		_, _ = s.src.Write(s.handshakeBuffer[resLen:])
	}
	s.handshakeBuffer = s.handshakeBuffer[:0]

	if s.upgradeResponseCallback != nil {
		s.upgradeResponseCallback(res)
	}

	if !IsUpgradeRes(res) {
		return ErrCannotUpgrade
	}

	if key := res.Header.Get("Sec-WebSocket-Accept"); key != expectedKey {
		return ErrCannotUpgrade
	}

	return nil
}

// makeHandshakeKey generates the key of Sec-WebSocket-Key header as well as the
// expected response present in Sec-WebSocket-Accept header.
func (s *Stream) makeHandshakeKey() (req, res string) {
	// request
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	req = base64.StdEncoding.EncodeToString(b)

	// response
	var resKey []byte
	resKey = append(resKey, []byte(req)...)
	resKey = append(resKey, GUID...)

	s.hasher.Reset()
	s.hasher.Write(resKey)
	res = base64.StdEncoding.EncodeToString(s.hasher.Sum(nil))

	return
}

// SetControlCallback sets a function that will be invoked when a Ping/Pong/Close is received while reading a
// message. This callback is only invoked when reading complete messages, not frames.
//
// The caller must not perform any operations on the stream in the provided callback.
func (s *Stream) SetControlCallback(controlCallback ControlCallback) {
	s.controlCallback = controlCallback
}

func (s *Stream) ControlCallback() ControlCallback {
	return s.controlCallback
}

// SetUpgradeRequestCallback sets a function that will be invoked during the handshake just before the upgrade request
// is sent.
//
// The caller must not perform any operations on the stream in the provided callback.
func (s *Stream) SetUpgradeRequestCallback(upgradeRequestCallback UpgradeRequestCallback) {
	s.upgradeRequestCallback = upgradeRequestCallback
}

func (s *Stream) UpgradeRequestCallback() UpgradeRequestCallback {
	return s.upgradeRequestCallback
}

// SetUpgradeResponseCallback sets a function that will be invoked during the handshake just after the upgrade response
// is received.
//
// The caller must not perform any operations on the stream in the provided callback.
func (s *Stream) SetUpgradeResponseCallback(upgradeResponseCallback UpgradeResponseCallback) {
	s.upgradeResponseCallback = upgradeResponseCallback
}

func (s *Stream) UpgradeResponseCallback() UpgradeResponseCallback {
	return s.upgradeResponseCallback
}

// SetMaxMessageSize sets the maximum size of a message that can be read from or written to a peer.
//
// - If a message exceeds the limit while reading, the connection is closed abnormally.
// - If a message exceeds the limit while writing, the operation is cancelled.
func (s *Stream) SetMaxMessageSize(bytes int) {
	// This is just for checking against the length returned in the frame
	// header. The sizes of the buffers in which we read or write the messages
	// are dynamically adjusted in frame_codec.
	s.maxMessageSize = bytes

	// SetMaxMessageSize might get called before init() where s.codec is initialized.
	if s.codec != nil {
		s.codec.maxMessageSize = bytes
	}
}

func (s *Stream) MaxMessageSize() int {
	return s.maxMessageSize
}

func (s *Stream) RemoteAddr() net.Addr {
	return s.conn.RemoteAddr()
}

func (s *Stream) LocalAddr() net.Addr {
	return s.conn.LocalAddr()
}

func (s *Stream) RawFd() int {
	if s.NextLayer() != nil {
		return s.NextLayer().(*sonic.AsyncAdapter).RawFd()
	}
	return -1
}

func (s *Stream) CloseNextLayer() (err error) {
	if s.conn != nil {
		err = s.conn.Close()
		s.conn = nil
	}
	return
}

// AsyncNextMessageDirect reads the next websocket message asynchronously, returning either
// one zero-copy slice (if the message fits in one frame) or multiple copied slices
// (if the message is fragmented).
//
// This call first flushes any pending control frames to the underlying stream asynchronously.
//
// This call does not block. The provided callback is invoked when one of the following happens:
//   - an error occurs while flushing the pending control frames
//   - an error occurs when reading/decoding the message bytes from the underlying stream
//   - the payload of the message is successfully read
func (s *Stream) AsyncNextMessageDirect(callback AsyncMessageDirectCallback) {
	s.AsyncFlush(func(err error) {
		if errors.Is(err, ErrMessageTooBig) {
			s.AsyncClose(CloseGoingAway, "payload too big", func(_ error) {})
			callback(ErrMessageTooBig, TypeNone)
			return
		}

		if err == nil && !s.canRead() {
			err = io.EOF
		}

		if err == nil {
			s.asyncNextMessageDirectFirstFrame(callback)
		} else {
			s.state = StateTerminated
			callback(err, TypeNone)
		}
	})
}

// asyncNextMessageDirectFirstFrame reads the first data frame.
// If it is FIN, we do single-frame zero-copy. Otherwise, we start gathering frames.
func (s *Stream) asyncNextMessageDirectFirstFrame(cb AsyncMessageDirectCallback) {
	s.AsyncNextFrame(func(err error, f Frame) {
		if err != nil {
			cb(err, TypeNone)
			return
		}
		if f.Opcode().IsControl() {
			// Handle control frames
			if s.controlCallback != nil {
				s.controlCallback(MessageType(f.Opcode()), f.Payload())
			}
			s.asyncNextMessageDirectFirstFrame(cb)
			return
		}

		mt := MessageType(f.Opcode())

		if f.PayloadLength() > s.maxMessageSize {
			// Close socket if payload too big
			s.AsyncClose(CloseGoingAway, "payload too big", func(_ error) {})
			cb(ErrMessageTooBig, mt)
			return
		}

		if f.IsFIN() {
			// If message is made of a single frame, return that frame's payload (zero copy)
			cb(nil, mt, f.Payload())
			return
		}

		// If message is made of a multiple frames, we must copy each frame's payload
		// This is because existing codec/ws stream designed around only holding on to one frame at a time
		payloadCopy := make([]byte, f.PayloadLength())
		copy(payloadCopy, f.Payload())

		parts := [][]byte{payloadCopy}
		s.asyncNextMessageDirectFragments(mt, parts, cb)
	})
}

// asyncNextMessageDirectFragments reads continuation frames until FIN, storing each payload in parts.
// For all frames that are not FIN, we do a copy into a new buffer. For the final frame (FIN=true),
// we append f.Payload() directly (zero-copy).
func (s *Stream) asyncNextMessageDirectFragments(
	messageType MessageType,
	parts [][]byte,
	cb AsyncMessageDirectCallback,
) {
	s.AsyncNextFrame(func(err error, f Frame) {
		if err != nil {
			cb(err, messageType, parts...)
			return
		}
		if f.Opcode().IsControl() {
			// Handle control frames, then keep reading data frames
			if s.controlCallback != nil {
				s.controlCallback(MessageType(f.Opcode()), f.Payload())
			}
			s.asyncNextMessageDirectFragments(messageType, parts, cb)
			return
		}
		if !f.Opcode().IsContinuation() {
			// If we encounter a protocol error, close the socket
			s.AsyncClose(CloseProtocolError, "expected continuation", func(_ error) {})
			cb(ErrExpectedContinuation, messageType, parts...)
			return
		}

		// Check total message size
		totalSoFar := 0
		for _, p := range parts {
			totalSoFar += len(p)
		}
		if totalSoFar+f.PayloadLength() > s.maxMessageSize {
			// Close socket if payload too big
			s.AsyncClose(CloseGoingAway, "payload too big", func(_ error) {})
			cb(ErrMessageTooBig, messageType, parts...)
			return
		}

		if f.IsFIN() {
			// If this is final fragment of message, return all of the payload parts
			// We don't need to copy this final fragment, for the same reason we don't for messages of a
			// single frame
			parts = append(parts, f.Payload())
			cb(nil, messageType, parts...)
			return
		}

		// Otherwise, copy this frame and keep reading next fragments
		payloadCopy := make([]byte, f.PayloadLength())
		copy(payloadCopy, f.Payload())
		parts = append(parts, payloadCopy)

		s.asyncNextMessageDirectFragments(messageType, parts, cb)
	})
}
