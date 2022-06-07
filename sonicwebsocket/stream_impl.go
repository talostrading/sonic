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

var _ Stream = &StreamImpl{}

type StreamImpl struct {
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
	readFrame *Frame

	// conn is the underlying tcp stream connection
	conn net.Conn

	// async makes it possible to execute asynchronous operations on conn
	async *sonic.AsyncAdapter

	readLimit uint64

	// make something nicer
	binary bool
	text   bool
}

func NewStreamImpl(ioc *sonic.IO, cb StateChangeCallback, tls *tls.Config) *StreamImpl {
	s := &StreamImpl{
		ioc:       ioc,
		state:     StateClosed,
		tls:       tls,
		readFrame: NewFrame(),
		writeBuf:  bytes.NewBuffer(make([]byte, 0, FrameHeaderSize+FrameMaskSize+DefaultFramePayloadSize)),
		readLimit: MaxPayloadSize,
	}
	return s
}

func (s *StreamImpl) dial(ctx context.Context, network, addr string) (net.Conn, error) {
	return s.conn, nil
}

func (s *StreamImpl) Read(b []byte) error {
	panic("implement me")
}

func (s *StreamImpl) ReadSome(b []byte) error {
	panic("implement me")
}

func (s *StreamImpl) AsyncRead(b []byte, cb sonic.AsyncCallback) {
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

func (s *StreamImpl) AsyncReadSome(b []byte, cb sonic.AsyncCallback) {
	s.asyncReadFrame(b, cb)
}

func (s *StreamImpl) asyncReadFrame(b []byte, cb sonic.AsyncCallback) {
	s.asyncReadFrameHeader(b, cb)
}
func (s *StreamImpl) asyncReadFrameHeader(b []byte, cb sonic.AsyncCallback) {
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

func (s *StreamImpl) asyncReadFrameExtraLength(m int, b []byte, cb sonic.AsyncCallback) {
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

func (s *StreamImpl) asyncReadFrameMask(b []byte, cb sonic.AsyncCallback) {
	s.async.AsyncReadAll(s.readFrame.mask[:4], func(err error, n int) {
		if err != nil {
			cb(ErrReadingMask, 0)
		} else {
			s.asyncReadPayload(b, cb)
		}
	})
}

func (s *StreamImpl) asyncReadPayload(b []byte, cb sonic.AsyncCallback) {
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

func (s *StreamImpl) Write(b []byte) error {
	panic("implement me")
}

// WriteSome writes some message data.
func (s *StreamImpl) WriteSome(fin bool, b []byte) error {
	panic("implement me")
}

// AsyncWrite writes a complete message asynchronously.
func (s *StreamImpl) AsyncWrite(b []byte, cb sonic.AsyncCallback) {
	fr := AcquireFrame()

	// TODO For a full fledged server, we would want the caller
	// to set the payload size per frame as an option first. Then
	// this function will break up the message into multiple frames is necessary.
	// For now, this works fine. We send everything in one frame.
	fr.SetFin()

	if s.binary {
		fr.SetBinary()
	} else if s.text {
		fr.SetText()
	}

	fr.SetPayload(b)

	// TODO introduce websocket role (only clients should mask the data, servers not)
	fr.Mask()

	s.asyncWriteFrame(fr, func(err error, n int) {
		cb(err, n)
		ReleaseFrame(fr)
	})
}

// AsyncWriteSome writes some message data asynchronously.
func (s *StreamImpl) AsyncWriteSome(fin bool, b []byte, cb sonic.AsyncCallback) {
	fr := AcquireFrame()

	if fin {
		fr.SetFin()
	}

	if s.binary {
		fr.SetBinary()
	} else if s.text {
		fr.SetText()
	}

	fr.SetPayload(b)

	// TODO introduce websocket role (only clients should mask the data, servers not)
	fr.Mask()

	s.asyncWriteFrame(fr, func(err error, n int) {
		cb(err, n)
		ReleaseFrame(fr)
	})
}

func (s *StreamImpl) asyncWriteFrame(fr *Frame, cb sonic.AsyncCallback) {
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
func (s *StreamImpl) SetReadLimit(limit uint64) {
	if limit == 0 {
		s.readLimit = MaxPayloadSize
	} else {
		s.readLimit = limit
	}
}

func (s *StreamImpl) State() StreamState {
	return s.state
}

func (s *StreamImpl) GotText() bool {
	return s.readFrame.IsText()
}

func (s *StreamImpl) GotBinary() bool {
	return s.readFrame.IsBinary()
}

func (s *StreamImpl) IsMessageDone() bool {
	return s.readFrame.IsContinuation()
}

func (s *StreamImpl) SendBinary(v bool) {
	if v {
		s.binary = true
		s.text = false
	} else {
		s.binary = false
		s.text = true
	}
}

func (s *StreamImpl) SentBinary() bool {
	return s.binary
}

func (s *StreamImpl) SendText(v bool) {
	if v {
		s.binary = false
		s.text = true
	} else {
		s.binary = true
		s.text = false
	}
}

func (s *StreamImpl) SentText() bool {
	return s.readFrame.IsText()
}

func (s *StreamImpl) SetControlCallback(ccb AsyncControlCallback) {
	s.ccb = ccb
}

func (s *StreamImpl) ControlCallback() AsyncControlCallback {
	return s.ccb
}

func (s *StreamImpl) Handshake(addr string) error {
	s.state = StateHandshake

	conn, sc, err := s.handshake(addr)
	if err != nil {
		return err
	}

	s.conn = conn

	done := make(chan struct{}, 1)
	sonic.NewAsyncAdapter(s.ioc, sc, s.conn, func(asyncErr error, async *sonic.AsyncAdapter) {
		if asyncErr != nil {
			err = asyncErr
		} else {
			err = nil
			s.async = async
		}
		done <- struct{}{}
	})
	<-done

	return err
}

func (s *StreamImpl) AsyncHandshake(addr string, cb func(error)) {
	s.state = StateHandshake

	go func() {
		conn, sc, err := s.handshake(addr)
		if err != nil {
			cb(err)
			return
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
	}()
}

func (s *StreamImpl) handshake(addr string) (conn net.Conn, sc syscall.Conn, err error) {
	uri, err := s.resolveUpgrader(addr)
	if err != nil {
		return nil, nil, err
	}

	if uri.Scheme == "http" {
		port := uri.Port()
		if port == "" {
			port = ":80"
		}
		conn, err = net.Dial("tcp", uri.Hostname()+port)
		if err != nil {
			return nil, nil, err
		}
		sc = conn.(syscall.Conn)
	} else {
		if s.tls == nil {
			return nil, nil, fmt.Errorf("wss requested with nil tls config")
		}
		port := uri.Port()
		if port == "" {
			port = ":443"
		}

		conn, err = tls.Dial("tcp", uri.Hostname()+port, s.tls)
		if err != nil {
			return nil, nil, err
		}
		sc = conn.(*tls.Conn).NetConn().(syscall.Conn)
	}
	if err != nil {
		return nil, nil, err
	}

	upgrader := &http.Client{
		Transport: &http.Transport{
			DialContext:     s.dial,
			DialTLSContext:  s.dial,
			TLSClientConfig: s.tls,
		},
	}

	req, err := http.NewRequest("GET", uri.String(), nil)
	if err != nil {
		return nil, nil, err
	}

	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Sec-WebSocket-Key", string(makeRandKey()))
	req.Header.Set("Sec-Websocket-Version", "13")

	res, err := upgrader.Do(req)
	if err != nil {
		return nil, nil, err
	}

	if res.StatusCode != 101 || res.Header.Get("Upgrade") != "websocket" {
		// TODO check the Sec-Websocket-Accept header as well
		return nil, nil, ErrCannotUpgrade
	}

	return conn, sc, nil
}

func (s *StreamImpl) resolveUpgrader(addr string) (*url.URL, error) {
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

func (s *StreamImpl) Accept() error {
	// TODO
	return nil
}

func (s *StreamImpl) AsyncAccept(func(error)) {
	// TODO
}

func (s *StreamImpl) Close(*CloseReason) error {
	// TODO
	return nil
}

func (s *StreamImpl) AsyncClose(*CloseReason, func(error)) {
	// TODO
}

func (s *StreamImpl) Ping(payload PingPongPayload) error {
	// TODO
	return nil
}

func (s *StreamImpl) AsyncPing(PingPongPayload, func(error)) {
	// TODO
}

func (s *StreamImpl) Pong(PingPongPayload) error {
	// TODO
	return nil
}

func (s *StreamImpl) AsyncPong(PingPongPayload, func(error)) {
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
