package sonicwebsocket

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"syscall"

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

	// readFrame is the frame in which read data is deserialized into
	readFrame *frame

	// conn is the underlying tcp stream connection
	conn net.Conn

	// async makes it possible to execute asynchronous operations on conn
	async *sonic.AsyncAdapter

	readLimit uint64

	// role of the WebsocketStream (client or server)
	role Role

	opts Options
}

func NewWebsocketStream(ioc *sonic.IO, tls *tls.Config, role Role, options ...Options) (Stream, error) {
	var opts Options
	for _, opt := range options {
		opts.Set(opt)
	}

	opts.validate()

	s := &WebsocketStream{
		ioc:       ioc,
		state:     StateClosed,
		tls:       tls,
		readFrame: newFrame(),
		writeBuf:  bytes.NewBuffer(make([]byte, 0, frameHeaderSize+frameMaskSize+DefaultPayloadSize)),
		readLimit: MaxPayloadSize,
		opts:      opts,
		role:      role,
	}

	return s, nil
}

func (s *WebsocketStream) Options() *Options {
	return &s.opts
}

func (s *WebsocketStream) NextLayer() sonic.Stream {
	return s.async
}

func (s *WebsocketStream) Read(b []byte) error {
	panic("implement me")
}

func (s *WebsocketStream) ReadSome(b []byte) error {
	panic("implement me")
}

func (s *WebsocketStream) AsyncRead(b []byte, cb sonic.AsyncCallback) {
	var read uint64 = 0
	for {
		if read >= s.readLimit {
			break
		}

		s.asyncReadFrame(b[read:s.readLimit], func(err error, n int) {
			if err != nil {
				cb(err, n)
			} else {
				read += uint64(n)
			}
		})

		if s.IsMessageDone() {
			break
		}
	}
}

func (s *WebsocketStream) AsyncReadSome(b []byte, cb sonic.AsyncCallback) {
	s.asyncReadFrame(b, cb)
}

func (s *WebsocketStream) asyncReadFrame(b []byte, cb sonic.AsyncCallback) {
	s.asyncReadFrameHeader(b, cb)
}
func (s *WebsocketStream) asyncReadFrameHeader(b []byte, cb sonic.AsyncCallback) {
	s.async.AsyncReadAll(s.readFrame.header[:2], func(err error, n int) {
		if err != nil {
			cb(ErrReadingHeader, 0)
		} else {
			m := s.readFrame.readMore()
			if m > 0 {
				s.asyncReadFrameExtraLength(m, b, cb)
			} else {
				if s.readFrame.IsMasked() {
					s.asyncReadFrameMask(b, cb)
				} else {
					s.asyncReadPayload(b, cb)
				}
			}
		}
	})
}

func (s *WebsocketStream) asyncReadFrameExtraLength(m int, b []byte, cb sonic.AsyncCallback) {
	s.async.AsyncReadAll(s.readFrame.header[2:m+2], func(err error, n int) {
		if err != nil {
			cb(ErrReadingExtendedLength, 0)
		} else {
			if s.readFrame.Len() > s.readLimit {
				cb(ErrPayloadTooBig, 0)
			} else {
				if s.readFrame.IsMasked() {
					s.asyncReadFrameMask(b, cb)
				} else {
					s.asyncReadPayload(b, cb)
				}
			}
		}
	})
}

func (s *WebsocketStream) asyncReadFrameMask(b []byte, cb sonic.AsyncCallback) {
	s.async.AsyncReadAll(s.readFrame.mask[:4], func(err error, n int) {
		if err != nil {
			cb(ErrReadingMask, 0)
		} else {
			s.asyncReadPayload(b, cb)
		}
	})
}

func (s *WebsocketStream) asyncReadPayload(b []byte, cb sonic.AsyncCallback) {
	if payloadLen := int(s.readFrame.Len()); payloadLen > 0 {
		if remaining := payloadLen - int(cap(b)); remaining > 0 {
			cb(ErrPayloadTooBig, 0)
		} else {
			b = b[:payloadLen]
			s.async.AsyncReadAll(b, func(err error, n int) {
				if err != nil {
					cb(err, n)
				} else {
					cb(nil, n)
				}
			})
		}
	} else {
		panic("invalid uint64 to int conversion")
	}
}

func (s *WebsocketStream) Write(b []byte) error {
	panic("implement me")
}

// WriteSome writes some message data.
func (s *WebsocketStream) WriteSome(fin bool, b []byte) error {
	panic("implement me")
}

// AsyncWrite writes a complete message asynchronously.
func (s *WebsocketStream) AsyncWrite(b []byte, cb sonic.AsyncCallback) {
	fr := s.makeFrame(true, b)

	s.asyncWriteFrame(fr, func(err error, n int) {
		cb(err, n)
		ReleaseFrame(fr)
	})
}

// AsyncWriteSome writes some message data asynchronously.
func (s *WebsocketStream) AsyncWriteSome(fin bool, b []byte, cb sonic.AsyncCallback) {
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

	if s.opts.IsSet(OptionText) {
		fr.SetText()
	} else if s.opts.IsSet(OptionBinary) {
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
	return s.readFrame.IsText()
}

func (s *WebsocketStream) GotBinary() bool {
	return s.readFrame.IsBinary()
}

func (s *WebsocketStream) IsMessageDone() bool {
	return s.readFrame.IsContinuation()
}

func (s *WebsocketStream) SentBinary() bool {
	return s.opts.IsSet(OptionBinary)
}

func (s *WebsocketStream) SentText() bool {
	return s.opts.IsSet(OptionText)
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
	// we can asynchronously dial endpoints and remove the need for a goroutine.
	go func() {
		s.handshake(addr, func(err error) {
			s.ioc.Dispatch(func() {
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
	} else {
		s.dial(uri, func(err error) {
			if err != nil {
				cb(err)
			} else {
				err = s.upgrade(uri)
				if err != nil {
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

		conn, err = tls.Dial("tcp", uri.Hostname()+":"+port, s.tls)
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

func (s *WebsocketStream) Close(*CloseReason) error {
	// TODO
	return nil
}

func (s *WebsocketStream) AsyncClose(*CloseReason, func(error)) {
	// TODO
}

func (s *WebsocketStream) Ping(payload PingPongPayload) error {
	// TODO
	return nil
}

func (s *WebsocketStream) AsyncPing(PingPongPayload, func(error)) {
	// TODO
}

func (s *WebsocketStream) Pong(PingPongPayload) error {
	// TODO
	return nil
}

func (s *WebsocketStream) AsyncPong(PingPongPayload, func(error)) {
	// TODO
}

func makeRandKey() []byte {
	b := make([]byte, 16)
	rand.Read(b[:])
	n := base64.StdEncoding.EncodedLen(16)
	key := make([]byte, n)
	base64.StdEncoding.Encode(key, b)
	return key
}
