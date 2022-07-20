package websocket

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"crypto/sha1"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"hash"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"syscall"

	"github.com/talostrading/sonic"
	"github.com/talostrading/sonic/sonicerrors"
)

var _ Stream = &WebsocketStream{}

type WebsocketStream struct {
	// Async operations executor.
	ioc *sonic.IO

	// User provided TLS config; nil if we don't use TLS
	tls *tls.Config

	// Underlying transport stream.
	stream sonic.Stream

	// Codec stream wrapping the underlying transport stream.
	cs *sonic.BlockingCodecStream[*Frame]

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

	// Contains the handshake response. Is emptied after the
	// handshake is over.
	hb []byte

	// Contains frames waiting to be sent to the peer.
	// Is emptied by AsyncFlush or Flush.
	pending []*Frame

	// Optional callback invoked when a control frame is received.
	ccb ControlCallback
}

func NewWebsocketStream(ioc *sonic.IO, tls *tls.Config, role Role) (*WebsocketStream, error) {
	s := &WebsocketStream{
		ioc:    ioc,
		tls:    tls,
		role:   role,
		src:    sonic.NewByteBuffer(),
		dst:    sonic.NewByteBuffer(),
		state:  StateHandshake,
		hasher: sha1.New(),
		hb:     make([]byte, 1024),
	}

	s.src.Reserve(4096)
	s.dst.Reserve(4096)

	return s, nil
}

// init is run when we transition into StateActive
func (s *WebsocketStream) init(stream sonic.Stream) (err error) {
	s.stream = stream
	codec := NewFrameCodec(s.src, s.dst)
	s.cs, err = sonic.NewBlockingCodecStream[*Frame](stream, codec, s.src, s.dst)
	return
}

func (s *WebsocketStream) NextLayer() sonic.Stream {
	return s.cs.NextLayer()
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
	if err == nil && !s.canRead() {
		err = io.EOF
	}

	if err == nil {
		f, err = s.nextFrame()
	}

	if err != nil {
		s.state = StateTerminated
	}

	return
}

func (s *WebsocketStream) nextFrame() (f *Frame, err error) {
	f, err = s.cs.ReadNext()
	if err == nil {
		err = s.handleFrame(f)
	}
	return
}

func (s *WebsocketStream) AsyncNextFrame(cb AsyncFrameHandler) {
	s.AsyncFlush(func(err error) {
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
	s.cs.AsyncReadNext(func(err error, f *Frame) {
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

			if n != f.PayloadLen() {
				err = ErrPayloadTooBig
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

				if n != f.PayloadLen() {
					err = ErrPayloadTooBig
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
		s.prepareClose(CloseProtocolError, "")
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
			// the peer closed the connection so we reply with the same
			// close frame
			s.state = StateClosedByPeer

			closeFrame := AcquireFrame()
			closeFrame.SetFin()
			closeFrame.SetClose()
			closeFrame.SetPayload(f.payload)
			if s.role == RoleClient {
				closeFrame.Mask()
			}
			s.pending = append(s.pending, closeFrame)
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
	f := AcquireFrame()
	f.SetFin()
	f.SetOpcode(Opcode(mt))
	f.SetPayload(b)

	return s.WriteFrame(f)
}

func (s *WebsocketStream) WriteFrame(f *Frame) error {
	if s.state == StateActive {
		s.prepareWrite(f)
		return s.Flush()
	} else {
		return ErrSendAfterClose
	}
}

func (s *WebsocketStream) AsyncWrite(b []byte, mt MessageType, cb func(err error)) {
	f := AcquireFrame()
	f.SetFin()
	f.SetOpcode(Opcode(mt))
	f.SetPayload(b)

	s.AsyncWriteFrame(f, cb)
}

func (s *WebsocketStream) AsyncWriteFrame(f *Frame, cb func(err error)) {
	if s.state == StateActive {
		s.prepareWrite(f)
		s.AsyncFlush(cb)
	} else {
		cb(ErrSendAfterClose)
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
		s.prepareClose(cc, reason)
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
		s.prepareClose(cc, reason)
		return s.Flush()
	case StateClosedByUs, StateHandshake:
		return sonicerrors.ErrCancelled
	default:
		return io.EOF
	}
}

func (s *WebsocketStream) prepareClose(cc CloseCode, reason string) {
	s.state = StateClosedByUs

	closeFrame := AcquireFrame()
	closeFrame.SetFin()
	closeFrame.SetClose()
	closeFrame.SetPayload(EncodeCloseFramePayload(cc, reason))
	if s.role == RoleClient {
		closeFrame.Mask()
	}

	s.pending = append(s.pending, closeFrame)
}

func (s *WebsocketStream) handle(fr *Frame) (n int, err error) {
	return
}

func (s *WebsocketStream) Flush() (err error) {
	flushed := 0
	for i := 0; i < len(s.pending); i++ {
		_, err = s.cs.WriteNext(s.pending[i])
		if err != nil {
			break
		}
		ReleaseFrame(s.pending[i])
		flushed = i
	}
	s.pending = s.pending[flushed:]
	return
}

func (s *WebsocketStream) AsyncFlush(cb func(err error)) {
	s.asyncFlush(0, cb)
}

func (s *WebsocketStream) asyncFlush(flushed int, cb func(err error)) {
	if flushed >= len(s.pending) {
		s.pending = s.pending[:0]
		cb(nil)
	} else {
		s.cs.AsyncWriteNext(s.pending[flushed], func(err error, _ int) {
			if err != nil {
				s.pending = s.pending[flushed:]
				cb(err)
			} else {
				ReleaseFrame(s.pending[flushed])
				s.asyncFlush(flushed+1, cb)
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

func (s *WebsocketStream) Handshake(addr string) (err error) {
	if s.role != RoleClient {
		return ErrWrongHandshakeRole
	}

	done := make(chan struct{}, 1)
	s.handshake(addr, func(herr error) {
		done <- struct{}{}
		err = herr
	})
	<-done
	return
}

func (s *WebsocketStream) AsyncHandshake(addr string, cb func(error)) {
	if s.role != RoleClient {
		cb(ErrWrongHandshakeRole)
		return
	}

	// I know, this is horrible, but if you help me write a TLS client for sonic
	// we can asynchronously dial endpoints and remove the need for a goroutine here
	go func() {
		s.handshake(addr, func(err error) {
			s.ioc.Post(func() {
				cb(err)
			})
		})
	}()
}

func (s *WebsocketStream) handshake(addr string, cb func(err error)) {
	s.state = StateHandshake

	url, err := s.resolve(addr)
	if err == nil {
		s.dial(url, func(err error) {
			if err == nil {
				err = s.upgrade(url)
				if err == nil {
					s.state = StateActive
				}
			}

			if err != nil {
				s.state = StateTerminated
			}

			cb(err)
		})
	} else {
		s.state = StateTerminated
		cb(err)
	}
}

func (s *WebsocketStream) resolve(addr string) (url *url.URL, err error) {
	url, err = url.Parse(addr)
	if err == nil {
		switch url.Scheme {
		case "ws":
			url.Scheme = "http"
		case "wss":
			url.Scheme = "https"
		default:
			err = fmt.Errorf("invalid address=%s", addr)
		}
	}

	return
}

func (s *WebsocketStream) dial(url *url.URL, cb func(err error)) {
	var (
		err  error
		conn net.Conn
		sc   syscall.Conn

		port = url.Port()
	)

	switch url.Scheme {
	case "http":
		if port == "" {
			port = "80"
		}
		addr := url.Hostname() + ":" + port
		conn, err = net.Dial("tcp", addr)
		if err == nil {
			sc = conn.(syscall.Conn)
		}
	case "https":
		if s.tls == nil {
			err = fmt.Errorf("wss:// scheme endpoints require a TLS configuration.")
		}

		if err == nil {
			if port == "" {
				port = "443"
			}
			addr := url.Hostname() + ":" + port
			conn, err = tls.Dial("tcp", addr, s.tls)
			if err == nil {
				sc = conn.(*tls.Conn).NetConn().(syscall.Conn)
			}
		}
	default:
		err = fmt.Errorf("invalid url scheme=%s", url.Scheme)
	}

	if err == nil {
		sonic.NewAsyncAdapter(s.ioc, sc, conn, func(err error, stream *sonic.AsyncAdapter) {
			if err == nil {
				err = s.init(stream)
			}
			cb(err)
		})
	} else {
		cb(err)
	}
}

func (s *WebsocketStream) upgrade(uri *url.URL) error {
	req, err := http.NewRequest("GET", uri.String(), nil)
	if err != nil {
		return err
	}

	sentKey, expectedKey := s.makeHandshakeKey()
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Connection", "upgrade")
	req.Header.Set("Sec-WebSocket-Key", string(sentKey))
	req.Header.Set("Sec-Websocket-Version", "13")

	err = req.Write(s.stream)
	if err != nil {
		return err
	}

	n, err := s.stream.Read(s.hb)
	if err != nil {
		return err
	}
	s.hb = s.hb[:n]
	rd := bytes.NewReader(s.hb)
	res, err := http.ReadResponse(bufio.NewReader(rd), req)
	if err != nil {
		return err
	}

	rawRes, err := httputil.DumpResponse(res, true)
	if err != nil {
		return err
	}

	resLen := len(rawRes)
	extra := len(s.hb) - resLen
	if extra > 0 {
		// we got some frames as well with the handshake so we can put
		// them in src for later decoding before clearing the handshake
		// buffer
		s.src.Write(s.hb[resLen:])
	}
	s.hb = s.hb[:0]

	if !IsUpgradeRes(res) {
		return ErrCannotUpgrade
	}

	if key := res.Header.Get("Sec-WebSocket-Accept"); key != expectedKey {
		return ErrCannotUpgrade
	}

	return nil
}

// makeHandshakeKey generates the key of Sec-WebSocket-Key header as well as the expected
// response present in Sec-WebSocket-Accept header.
func (s *WebsocketStream) makeHandshakeKey() (req, res string) {
	// request
	b := make([]byte, 16)
	rand.Read(b)
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

func (s *WebsocketStream) Accept() error {
	panic("implement me")
}

func (s *WebsocketStream) AsyncAccept(func(error)) {
	panic("implement me")
}

func (s *WebsocketStream) SetControlCallback(ccb ControlCallback) {
	s.ccb = ccb
}

func (s *WebsocketStream) ControlCallback() ControlCallback {
	return s.ccb
}
