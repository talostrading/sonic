package sonicwebsocket

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"syscall"
	"time"

	"github.com/talostrading/sonic"
)

var _ Stream = &WebsocketStream{}

// WebsocketStream is a stateful full-duplex connection between two endpoints which
// adheres to the WebSocket protocol.
//
// The WebsocketStream implements Stream and can be used by both clients and servers.
//
// The underlying layer through which all IO is done is non-blocking.
type WebsocketStream struct {
	// ioc is the object which executes async operations on behalf of StreamImpl
	ioc *sonic.IO

	// state is the current stream state
	state StreamState

	// tls is the TLS config used when establishing connections to endpoints
	// with the `wss` scheme
	tls *tls.Config

	// ccb is the callback called when a control frame is received
	ccb AsyncControlCallback

	writeBuf *bytes.Buffer

	// frame is the data frame in which read data is deserialized into.
	frame *frame

	// controlFramePaylaod contains the payload of the last read control frame.
	controlFramePayload []byte

	// conn is the underlying tcp stream connection
	conn net.Conn

	// async makes it possible to execute asynchronous operations on conn
	async *sonic.AsyncAdapter

	readLimit uint64

	// role of the WebsocketStream (client or server)
	role Role

	// text indicates whether we are sending text or binary data frames
	text bool

	// closeTimer is a timer which closes the underlying stream on expiry in the client role.
	// The expected behaviour is for the server to close the connection such that the client receives an io.EOF.
	// If that does not happen, this timer does it for the client.
	closeTimer *sonic.Timer
}

func NewWebsocketStream(ioc *sonic.IO, tls *tls.Config, role Role) (Stream, error) {
	closeTimer, err := sonic.NewTimer(ioc)
	if err != nil {
		return nil, err
	}

	s := &WebsocketStream{
		ioc:                 ioc,
		state:               StateClosed,
		tls:                 tls,
		writeBuf:            bytes.NewBuffer(make([]byte, 0, frameHeaderSize+frameMaskSize+DefaultPayloadSize)),
		readLimit:           MaxPayloadSize,
		text:                true,
		role:                role,
		frame:               newFrame(),
		controlFramePayload: make([]byte, MaxControlFramePayloadSize),
		closeTimer:          closeTimer,
	}

	return s, nil
}

func (s *WebsocketStream) NextLayer() sonic.Stream {
	return s.async
}

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

	s.frame.Reset()
	nn, err := s.frame.ReadFrom(s.async)
	return int(nn), err
}

func (s *WebsocketStream) AsyncRead(b []byte, cb sonic.AsyncCallback) {
	if s.state == StateClosed {
		cb(io.EOF, 0)
		return
	}

	s.asyncRead(b, 0, cb)
}

func (s *WebsocketStream) asyncRead(b []byte, readBytes int, cb sonic.AsyncCallback) {
	s.AsyncReadSome(b[readBytes:], func(err error, n int) {
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
	s.frame.Reset()
	s.asyncReadFrameHeader(b, cb)
}
func (s *WebsocketStream) asyncReadFrameHeader(b []byte, cb sonic.AsyncCallback) {
	s.async.AsyncReadAll(s.frame.header[:2], func(err error, n int) {
		if err != nil {
			cb(ErrReadingHeader, 0)
		} else {
			if s.frame.IsControl() {
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
			s.frame.String()))
	}

	m := s.frame.readMore()
	if m > 0 {
		panic(fmt.Errorf(
			"sonic-websocket: invalid control frame - length is more than 125 bytes frame=%s",
			s.frame.String()))
	} else {
		var ft FrameType
		switch s.frame.Opcode() {
		case OpcodePing:
			ft = Ping
		case OpcodePong:
			ft = Pong
		case OpcodeClose:
			ft = Close
		default:
			panic(fmt.Errorf(
				"sonic-websocket: invalid control frame - unknown opcode (not ping/pong/close) frame=%s",
				s.frame.String()))
		}

		s.asyncReadPayload(s.controlFramePayload, func(err error, n int) {
			if err != nil {
				panic("sonic-websocket: could not read control frame payload")
			} else {
				s.controlFramePayload = s.controlFramePayload[:n]
				s.handleControlFrame(ft, bTransparent, cbTransparent)
			}
		})
	}
}

func (s *WebsocketStream) handleControlFrame(ft FrameType, bTransparent []byte, cbTransparent sonic.AsyncCallback) {
	switch ft {
	case Close:
		s.handleClose()
	case Ping:
		s.handlePing()
	case Pong:
		s.handlePong()
	}

	s.AsyncRead(bTransparent, cbTransparent)
}

func (s *WebsocketStream) handleClose() {
	switch s.state {
	case StateHandshake:
		// Not possible.
	case StateOpen:
		// Received a close frame - MUST send a close frame back.
		// Note that there is no guarantee that any in-flight
		// writes will complete at this point.
		s.state = StateClosing

		closeFrame := AcquireFrame()
		closeFrame.SetPayload(s.frame.Payload())
		closeFrame.SetOpcode(OpcodeClose)
		closeFrame.SetFin()
		if s.role == RoleClient {
			closeFrame.Mask()
		}

		s.asyncWriteFrame(closeFrame, func(err error, _ int) {
			if err != nil {
				panic(fmt.Errorf("sonic-websocket: could not send close reply err=%v", err))
			} else {
				s.state = StateClosed

				if s.role == RoleServer {
					s.NextLayer().Close()
				} else {
					s.closeTimer.Arm(5*time.Second, func() {
						s.NextLayer().Close()
						s.state = StateClosed
					})
				}
			}
			ReleaseFrame(closeFrame)
		})
	case StateClosing:
		// TODO need a helper function or change handler signature to parse the reason for close which is in the first 2 bytes of the frame.
		if s.ccb != nil {
			s.ccb(Close, s.frame.Payload())
		}
		s.NextLayer().Close()
		s.state = StateClosed
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
		pongFrame.SetPayload(s.frame.Payload())

		s.asyncWriteFrame(pongFrame, func(err error, n int) {
			if err != nil {
				// TODO retry timer?
				panic("sonic-websocket: could not send pong frame")
			}
			ReleaseFrame(pongFrame)
		})

		if s.ccb != nil {
			s.ccb(Ping, s.frame.Payload())
		}
	}
}

func (s *WebsocketStream) handlePong() {
	if s.state == StateOpen {
		if s.ccb != nil {
			s.ccb(Pong, s.frame.Payload())
		}
	}
}

func (s *WebsocketStream) asyncReadDataFrame(b []byte, cb sonic.AsyncCallback) {
	m := s.frame.readMore()
	if m > 0 {
		s.asyncReadFrameExtraLength(m, b, cb)
	} else {
		if s.frame.IsMasked() {
			s.asyncReadFrameMask(b, cb)
		} else {
			s.asyncReadPayload(b, cb)
		}
	}
}

func (s *WebsocketStream) asyncReadFrameExtraLength(m int, b []byte, cb sonic.AsyncCallback) {
	s.async.AsyncReadAll(s.frame.header[2:m+2], func(err error, n int) {
		if err != nil {
			cb(ErrReadingExtendedLength, 0)
		} else {
			if s.frame.Len() > s.readLimit {
				cb(ErrPayloadTooBig, 0)
			} else {
				if s.frame.IsMasked() {
					s.asyncReadFrameMask(b, cb)
				} else {
					s.asyncReadPayload(b, cb)
				}
			}
		}
	})
}

func (s *WebsocketStream) asyncReadFrameMask(b []byte, cb sonic.AsyncCallback) {
	s.async.AsyncReadAll(s.frame.mask[:4], func(err error, n int) {
		if err != nil {
			cb(ErrReadingMask, 0)
		} else {
			s.asyncReadPayload(b, cb)
		}
	})
}

func (s *WebsocketStream) asyncReadPayload(b []byte, cb sonic.AsyncCallback) {
	if pl := s.frame.Len(); pl > 0 {
		payloadLen := int64(pl)
		if remaining := payloadLen - int64(cap(b)); remaining > 0 {
			cb(ErrPayloadTooBig, 0)
		} else {
			s.async.AsyncReadAll(b[:payloadLen], func(err error, n int) {
				if err != nil {
					cb(err, n)
				} else {
					cb(nil, n)
				}
			})
		}
	}
}

func (s *WebsocketStream) Write(b []byte) (n int, err error) {
	if s.state != StateOpen {
		return 0, io.EOF
	}

	fr := s.makeFrame(true, b)
	nn, err := fr.WriteTo(s.async)
	ReleaseFrame(fr)
	return int(nn), err
}

// WriteSome writes some message data.
func (s *WebsocketStream) WriteSome(fin bool, b []byte) (n int, err error) {
	if s.state != StateOpen {
		return 0, io.EOF
	}

	fr := s.makeFrame(fin, b)
	nn, err := fr.WriteTo(s.async)
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

func (s *WebsocketStream) makeFrame(fin bool, payload []byte) *frame {
	fr := AcquireFrame()

	if fin {
		fr.SetFin()
	}

	if s.text {
		fr.SetText()
	} else {
		fr.SetBinary()
	}

	fr.SetPayload(payload)

	if s.role == RoleClient {
		fr.Mask()
	}

	return fr
}

func (s *WebsocketStream) asyncWriteFrame(fr *frame, cb sonic.AsyncCallback) {
	s.writeBuf.Reset()

	nn, err := fr.WriteTo(s.writeBuf)
	n := int(nn)
	if err != nil {
		cb(err, n)
	} else {
		b := s.writeBuf.Bytes()
		b = b[:n]
		s.async.AsyncWriteAll(b, cb)
	}
}

// SetReadLimit sets the maximum read size. If 0, the max size is used.
func (s *WebsocketStream) SetReadLimit(limit uint64) {
	if limit == 0 {
		s.readLimit = MaxPayloadSize
	} else {
		s.readLimit = limit
	}
}

func (s *WebsocketStream) State() StreamState {
	return s.state
}

func (s *WebsocketStream) GotText() bool {
	return s.frame.IsText()
}

func (s *WebsocketStream) GotBinary() bool {
	return s.frame.IsBinary()
}

func (s *WebsocketStream) IsMessageDone() bool {
	return s.frame.IsFin()
}

func (s *WebsocketStream) SentBinary() bool {
	return !s.text
}

func (s *WebsocketStream) SentText() bool {
	return s.text
}

func (s *WebsocketStream) SendText(v bool) {
	s.text = v
}

func (s *WebsocketStream) SendBinary(v bool) {
	s.text = !v
}

func (s *WebsocketStream) SetControlCallback(ccb AsyncControlCallback) {
	s.ccb = ccb
}

func (s *WebsocketStream) ControlCallback() AsyncControlCallback {
	return s.ccb
}

func (s *WebsocketStream) Handshake(addr string) (err error) {
	done := make(chan struct{}, 1)
	s.handshake(addr, func(herr error) {
		err = herr
	})
	<-done
	return
}

func (s *WebsocketStream) AsyncHandshake(addr string, cb func(error)) {
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

func (s *WebsocketStream) handshake(addr string, cb func(error)) {
	s.state = StateHandshake

	uri, err := s.resolve(addr)
	if err != nil {
		cb(err)
		s.state = StateClosed
	} else {
		s.dial(uri, func(err error) {
			if err != nil {
				cb(err)
				s.state = StateClosed
			} else {
				err = s.upgrade(uri)
				if err != nil {
					s.state = StateClosed
				} else {
					s.state = StateOpen
				}
				cb(err)
			}
		})
	}
}

func (s *WebsocketStream) resolve(addr string) (*url.URL, error) {
	uri, err := url.Parse(addr)
	if err != nil {
		return nil, err
	}

	var scheme string

	switch uri.Scheme {
	case "ws":
		scheme = "http"
	case "wss":
		scheme = "https"
	default:
		return nil, fmt.Errorf("invalid address %s", addr)
	}

	return url.Parse(strings.Join([]string{scheme, "://", uri.Host, uri.Path}, ""))
}

func (s *WebsocketStream) dial(uri *url.URL, cb func(err error)) {
	var conn net.Conn
	var sc syscall.Conn
	var err error

	if uri.Scheme == "http" {
		port := uri.Port()
		if port == "" {
			port = "80"
		}
		conn, err = net.Dial("tcp", uri.Hostname()+":"+port)
		if err != nil {
			cb(err)
			return
		}
		sc = conn.(syscall.Conn)
	} else {
		if s.tls == nil {
			cb(fmt.Errorf("wss requested with nil tls config"))
			return
		}
		port := uri.Port()
		if port == "" {
			port = "443"
		}

		conn, err = tls.Dial("tcp", uri.Hostname()+":"+port, s.tls) // TODO dial timeout
		if err != nil {
			cb(err)
			return
		}
		sc = conn.(*tls.Conn).NetConn().(syscall.Conn)
	}

	s.conn = conn

	sonic.NewAsyncAdapter(s.ioc, sc, s.conn, func(err error, async *sonic.AsyncAdapter) {
		if err != nil {
			cb(err)
		} else {
			s.async = async
			cb(nil)
		}
	})
}

func (s *WebsocketStream) upgrade(uri *url.URL) error {
	upgrader := &http.Client{
		Transport: &http.Transport{
			DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
				return s.conn, nil
			},
			DialTLSContext: func(_ context.Context, _, _ string) (net.Conn, error) {
				return s.conn, nil
			},
			TLSClientConfig: s.tls,
		},
	}

	req, err := http.NewRequest("GET", uri.String(), nil)
	if err != nil {
		return err
	}

	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Sec-WebSocket-Key", string(makeRandKey()))
	req.Header.Set("Sec-Websocket-Version", "13")

	res, err := upgrader.Do(req)
	if err != nil {
		return err
	}

	if res.StatusCode != 101 || res.Header.Get("Upgrade") != "websocket" {
		// TODO check the Sec-Websocket-Accept header as well
		return ErrCannotUpgrade
	}

	return nil
}

func (s *WebsocketStream) Accept() error {
	// TODO
	return nil
}

func (s *WebsocketStream) AsyncAccept(func(error)) {
	// TODO
}

func (s *WebsocketStream) Close(cc CloseCode, reason ...string) error {
	// TODO not correct
	if s.state != StateClosed {
		s.state = StateClosed
		return s.async.Close()
	}
	return nil
}

func (s *WebsocketStream) Closed() bool {
	return s.state == StateClosed
}

func (s *WebsocketStream) Ping(b []byte) error {
	if len(b) > MaxControlFramePayloadSize {
		return ErrPayloadTooBig
	}
	return nil
}

func (s *WebsocketStream) AsyncPing(b []byte, cb func(error)) {
	if len(b) > MaxControlFramePayloadSize {
		cb(ErrPayloadTooBig)
	}
}

func (s *WebsocketStream) Pong(b []byte) error {
	if len(b) > MaxControlFramePayloadSize {
		return ErrPayloadTooBig
	}
	return nil
}

func (s *WebsocketStream) AsyncPong(b []byte, cb func(error)) {
	if len(b) > MaxControlFramePayloadSize {
		cb(ErrPayloadTooBig)
	}
}

func makeRandKey() []byte {
	b := make([]byte, 16)
	rand.Read(b[:])
	n := base64.StdEncoding.EncodedLen(16)
	key := make([]byte, n)
	base64.StdEncoding.Encode(key, b)
	return key
}
