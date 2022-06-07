package sonicwebsocket

import (
	"bytes"
	"context"
	"crypto/tls"
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

	buf *bytes.Buffer
	fr  *Frame

	// conn is the underlying tcp stream connection
	conn net.Conn

	// async makes it possible to execute asynchronous operations on conn
	async *sonic.AsyncAdapter
}

func NewStreamImpl(ioc *sonic.IO, cb StateChangeCallback, tls *tls.Config) *StreamImpl {
	s := &StreamImpl{
		ioc:   ioc,
		state: StateClosed,
		tls:   tls,
		fr:    NewFrame(),
		buf:   bytes.NewBuffer(make([]byte, 0, FrameHeaderSize+FrameMaskSize+DefaultFramePayloadSize)),
	}
	return s
}

func (s *StreamImpl) dial(ctx context.Context, network, addr string) (net.Conn, error) {
	return s.conn, nil
}

func (s *StreamImpl) AsyncRead(b []byte, cb sonic.AsyncCallback) {
	s.asyncReadFrame(b, cb)
}

func (s *StreamImpl) AsyncReadAll(b []byte, cb sonic.AsyncCallback) {
	read := 0
	for {
		if uint64(read) > MaxFramePayloadLen {
			cb(ErrPayloadTooBig, read)
			break
		}

		s.asyncReadFrame(b[read:], func(err error, n int) {
			if err != nil {
				cb(err, n)
			} else {
				read += n
			}
		})

		if s.IsMessageDone() {
			break
		}
	}
}

func (s *StreamImpl) asyncReadFrame(b []byte, cb sonic.AsyncCallback) {
	s.asyncReadFrameHeader(b, cb)
}
func (s *StreamImpl) asyncReadFrameHeader(b []byte, cb sonic.AsyncCallback) {
	s.async.AsyncReadAll(s.fr.header[:2], func(err error, n int) {
		if err != nil {
			cb(ErrReadingHeader, 0)
		} else {
			m := s.fr.readMore()
			if m > 0 {
				s.asyncReadFrameExtraLength(m, b, cb)
			} else {
				if s.fr.IsMasked() {
					s.asyncReadFrameMask(b, cb)
				} else {
					s.asyncReadPayload(b, cb)
				}
			}
		}
	})
}

func (s *StreamImpl) asyncReadFrameExtraLength(m int, b []byte, cb sonic.AsyncCallback) {
	s.async.AsyncReadAll(s.fr.header[2:m+2], func(err error, n int) {
		if err != nil {
			cb(ErrReadingExtendedLength, 0)
		} else {
			if s.fr.Len() > MaxFramePayloadLen {
				cb(ErrPayloadTooBig, 0)
			} else {
				if s.fr.IsMasked() {
					s.asyncReadFrameMask(b, cb)
				} else {
					s.asyncReadPayload(b, cb)
				}
			}
		}
	})
}

func (s *StreamImpl) asyncReadFrameMask(b []byte, cb sonic.AsyncCallback) {
	s.async.AsyncReadAll(s.fr.mask[:4], func(err error, n int) {
		if err != nil {
			cb(ErrReadingMask, 0)
		} else {
			s.asyncReadPayload(b, cb)
		}
	})
}

func (s *StreamImpl) asyncReadPayload(b []byte, cb sonic.AsyncCallback) {
	if payloadLen := int(s.fr.Len()); payloadLen > 0 {
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

func (s *StreamImpl) AsyncWrite(b []byte, cb sonic.AsyncCallback) {

}

func (s *StreamImpl) AsyncWriteAll(b []byte, cb sonic.AsyncCallback) {

}

func (s *StreamImpl) State() StreamState {
	return s.state
}

func (s *StreamImpl) GotText() bool {
	return s.fr.IsText()
}

func (s *StreamImpl) GotBinary() bool {
	return s.fr.IsBinary()
}

func (s *StreamImpl) IsMessageDone() bool {
	return s.fr.IsContinuation()
}

func (s *StreamImpl) SendBinary(v bool) {
	if v {
		s.fr.SetBinary()
	}
}

func (s *StreamImpl) SentBinary() bool {
	return s.fr.IsBinary()
}

func (s *StreamImpl) SendText(v bool) {
	if v {
		s.fr.SetText()
	}
}

func (s *StreamImpl) SentText() bool {
	return s.fr.IsText()
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
