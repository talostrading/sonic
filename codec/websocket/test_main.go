package websocket

import (
	"bufio"
	"bytes"
	"crypto/sha1"
	"fmt"
	"github.com/talostrading/sonic/sonicopts"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"strings"
	"sync/atomic"
	"time"

	"github.com/talostrading/sonic"
)

// MockServer is a server which can be used to test the WebSocket client.
type MockServer struct {
	ln     net.Listener
	conn   net.Conn
	closed int32
}

func (s *MockServer) Accept(addr string) (err error) {
	s.ln, err = net.Listen("tcp", addr)
	if err != nil {
		return err
	}

	conn, err := s.ln.Accept()
	if err != nil {
		return err
	}
	s.conn = conn

	b := make([]byte, 4096)
	n, err := s.conn.Read(b)
	if err != nil {
		return err
	}
	b = b[:n]

	req, err := http.ReadRequest(bufio.NewReader(bytes.NewBuffer(b)))
	if err != nil {
		return err
	}

	if !strings.EqualFold(req.Header.Get("Upgrade"), "websocket") {
		reqb, err := httputil.DumpRequest(req, true)
		if err == nil {
			err = fmt.Errorf("request is not websocket upgrade: %s", string(reqb))
		}
		return err
	}

	res := bytes.NewBuffer(nil)
	res.Write([]byte("HTTP/1.1 101 Switching Protocols\r\n"))
	res.Write([]byte("Upgrade: websocket\r\n"))
	res.Write([]byte("Connection: Upgrade\r\n"))
	res.Write([]byte(
		fmt.Sprintf("Sec-WebSocket-Accept: %s\r\n",
			MakeServerResponseKey(sha1.New(), []byte(req.Header.Get("Sec-WebSocket-Key"))))))
	res.Write([]byte("\r\n"))

	_, err = res.WriteTo(s.conn)

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
	// The underlying ByteBuffer return io.EOF if there is nothing to be read. Since we mock a blocking conn,
	// we must wait on any io.EOF, and that's what we emulate here.
	defer s.b.ConsumeAll()

	for {
		s.b.CommitAll()

		n, err = s.b.Read(b)
		if err != io.EOF {
			return
		}
	}
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
