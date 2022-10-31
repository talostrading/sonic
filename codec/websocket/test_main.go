package websocket

import (
	"crypto/sha1"
	"fmt"
	http2 "github.com/talostrading/sonic/codec/http"
	"github.com/talostrading/sonic/sonicopts"
	"net"
	"strings"
	"sync/atomic"
	"time"

	"github.com/talostrading/sonic"
)

func makeConn(ioc *sonic.IO, addr string) (sonic.Conn, error) {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return nil, err
	}
	adapter, err := sonic.AdaptNetConn(ioc, conn)
	if err != nil {
		return nil, err
	}
	return adapter, nil
}

// MockServer is a server which can be used to test the WebSocket client.
type MockServer struct {
	ln     net.Listener
	conn   net.Conn
	closed int32

	enc *http2.ResponseEncoder
	dec *http2.RequestDecoder
}

func NewMockServer(addr string) (*MockServer, error) {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}

	enc, err := http2.NewResponseEncoder()
	if err != nil {
		return nil, err
	}
	dec, err := http2.NewRequestDecoder()
	if err != nil {
		return nil, err
	}

	return &MockServer{ln: ln, enc: enc, dec: dec}, nil
}

func (s *MockServer) Accept() (err error) {
	conn, err := s.ln.Accept()
	if err != nil {
		return err
	}
	s.conn = conn

	b := sonic.NewByteBuffer()
	_, err = b.ReadFrom(conn)
	if err != nil {
		return err
	}

	req, err := s.dec.Decode(b)
	if err != nil {
		return err
	}

	if !strings.EqualFold(req.Header.Get("Upgrade"), "websocket") {
		return fmt.Errorf("malformed request: %s", string(b.Data()))
	}

	res, err := http2.NewResponse()
	if err != nil {
		return err
	}

	res.Proto = http2.ProtoHttp11
	res.StatusCode = http2.StatusSwitchingProtocols
	res.Status = http2.StatusText(http2.StatusSwitchingProtocols)
	res.Header.Add("Upgrade", "websocket")
	res.Header.Add("Connection", "Upgrade")
	res.Header.Add(
		"Sec-WebSocket-Accept",
		MakeServerResponseKey(sha1.New(), []byte(req.Header.Get("Sec-WebSocket-Key"))))

	b.Reset()
	err = s.enc.Encode(res, b)
	if err != nil {
		return err
	}

	_, err = b.WriteTo(s.conn)
	if err != nil {
		return err
	}

	return nil
}

func (s *MockServer) Write(b []byte) error {
	fr := AcquireFrame()
	defer ReleaseFrame(fr)

	fr.SetText()
	fr.SetPayload(b)
	fr.SetFin()

	_, err := fr.WriteTo(s.conn)
	return err
}

func (s *MockServer) Read(b []byte) error {
	fr := AcquireFrame()
	defer ReleaseFrame(fr)

	_, err := fr.ReadFrom(s.conn)
	if err == nil {
		copy(b, fr.Payload())
	}
	return err
}

func (s *MockServer) Close() {
	if atomic.CompareAndSwapInt32(&s.closed, 0, 1) {
		s.conn.Close()
		s.ln.Close()
	}
}

func (s *MockServer) IsClosed() bool {
	return atomic.LoadInt32(&s.closed) == 1
}

var _ sonic.Conn = &MockConn{}

// MockConn is a mock TCP connection that's not attached to any operating system
// IO executor. It is used to test reads and writes for WebSocket servers and
// clients.
//
// A WebsocketStream can be set to use a MockConn only if it is in
// StateActive, which occurs after a successful HandshakeClient or a call to init().
type MockConn struct {
	b *sonic.ByteBuffer
}

func NewMockConn() *MockConn {
	s := &MockConn{
		b: sonic.NewByteBuffer(),
	}
	return s
}

func (s *MockConn) Read(b []byte) (n int, err error) {
	return s.b.Read(b)
}

func (s *MockConn) AsyncRead(b []byte, cb sonic.AsyncCallback) {
	n, err := s.b.Read(b)
	cb(err, n)
}

func (s *MockConn) AsyncReadAll(b []byte, cb sonic.AsyncCallback) {
	n, err := s.b.Read(b)
	cb(err, n)
}

func (s *MockConn) Write(b []byte) (n int, err error) {
	return s.b.Write(b)
}

func (s *MockConn) AsyncWrite(b []byte, cb sonic.AsyncCallback) {
	n, err := s.b.Write(b)
	cb(err, n)
}

func (s *MockConn) AsyncWriteAll(b []byte, cb sonic.AsyncCallback) {
	n, err := s.b.Write(b)
	cb(err, n)
}

func (s *MockConn) RawFd() int                         { return -1 }
func (s *MockConn) CancelReads()                       {}
func (s *MockConn) CancelWrites()                      {}
func (s *MockConn) Closed() bool                       { return false }
func (s *MockConn) Close() error                       { return nil }
func (s *MockConn) Opts() []sonicopts.Option           { return nil }
func (s *MockConn) LocalAddr() net.Addr                { return nil }
func (s *MockConn) RemoteAddr() net.Addr               { return nil }
func (s *MockConn) SetDeadline(t time.Time) error      { return nil }
func (s *MockConn) SetReadDeadline(t time.Time) error  { return nil }
func (s *MockConn) SetWriteDeadline(t time.Time) error { return nil }
