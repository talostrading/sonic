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
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"syscall"

	"github.com/talostrading/sonic"
)

var _ Stream = &WebsocketStream{}

type WebsocketStream struct {
	ioc *sonic.IO   // async operations executor
	tls *tls.Config // nil if we don't use TLS

	role Role // are we client or server

	src *sonic.ByteBuffer // buffer for stream reads
	dst *sonic.ByteBuffer // buffer for stream writes

	state StreamState // state of the stream

	pending []*Frame // frames pending to be written on the wire

	hasher hash.Hash // hashes the Sec-Websocket-Key when the stream is a client

	hb []byte // handshake buffer - the handshake response is read into this buffer

	lastFrame *Frame

	stream sonic.Stream
	cs     *sonic.BlockingCodecStream[Frame]
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

	s.src.Prepare(4096)
	s.dst.Prepare(4096)

	return s, nil
}

// init is run when we transition into StateActive
func (s *WebsocketStream) init(stream sonic.Stream) (err error) {
	s.stream = stream
	codec := NewFrameCodec(s.src, s.dst)
	s.cs, err = sonic.NewBlockingCodecStream[Frame](stream, codec, s.src, s.dst)
	return
}

func (s *WebsocketStream) NextLayer() sonic.Stream {
	return s.cs.NextLayer()
}

func (s *WebsocketStream) DeflateSupported() bool {
	return false
}

func (s *WebsocketStream) ReadFrame() (*Frame, error) {
	f, err := s.cs.ReadNext()
	s.lastFrame = f
	return f, err
}

func (s *WebsocketStream) AsyncReadFrame(cb func(error, *Frame)) {
	s.cs.AsyncReadNext(func(err error, f *Frame) {
		s.lastFrame = f
		cb(err, f)
	})
}

func (s *WebsocketStream) Read(b []byte) (int, error) {
	readBytes := 0

	for {
		f, err := s.ReadFrame()
		if err != nil {
			return readBytes, err
		}

		n := copy(b[readBytes:], f.Payload())
		readBytes += n

		if n != f.PayloadLen() {
			return readBytes, ErrPayloadTooBig
		}

		if f.IsFin() {
			break
		}
	}

	return readBytes, nil
}

func (s *WebsocketStream) AsyncRead(b []byte, cb sonic.AsyncCallback) {
	s.asyncRead(b, 0, cb)
}

func (s *WebsocketStream) asyncRead(b []byte, readBytes int, cb sonic.AsyncCallback) {
	s.AsyncReadFrame(func(err error, f *Frame) {
		if err != nil {
			cb(err, readBytes)
		} else {
			n := copy(b[readBytes:], f.Payload())
			readBytes += n

			if n != f.PayloadLen() {
				cb(ErrPayloadTooBig, readBytes)
			} else {
				s.asyncRead(b, readBytes, cb)
			}
		}
	})
}

func (s *WebsocketStream) WriteFrame(f *Frame) (n int, err error) {
	return s.cs.WriteNext(f)
}

func (s *WebsocketStream) AsyncWriteFrame(f *Frame, cb sonic.AsyncCallback) {
	s.cs.AsyncWriteNext(f, cb)
}

func (s *WebsocketStream) handle(fr *Frame) (n int, err error) {
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
	return MessageType(s.lastFrame.Opcode())
}
