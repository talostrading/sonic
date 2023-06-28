package websocket

import (
	"bufio"
	"bytes"
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"sync/atomic"

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

	if !IsUpgradeReq(req) {
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
	res.Write([]byte(fmt.Sprintf("Sec-WebSocket-Accept: %s\r\n", MakeResponseKey([]byte(req.Header.Get("Sec-WebSocket-Key"))))))
	res.Write([]byte("\r\n"))

	_, err = res.WriteTo(s.conn)
	return err
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
		if s.conn != nil {
			_ = s.conn.Close()
		}
		if s.ln != nil {
			_ = s.ln.Close()
		}
	}
}

func (s *MockServer) IsClosed() bool {
	return atomic.LoadInt32(&s.closed) == 1
}

var _ sonic.Stream = &MockStream{}

// MockStream is a mock TCP stream that's not attached to any operating system
// IO executor. It is used to test reads and writes for WebSocket servers and
// clients.
//
// A WebsocketStream can be set to use a MockStream only if it is in
// StateActive, which occurs after a successful handshake or a call to init().
type MockStream struct {
	b *sonic.ByteBuffer
}

func NewMockStream() *MockStream {
	s := &MockStream{
		b: sonic.NewByteBuffer(),
	}
	return s
}

func (s *MockStream) Read(b []byte) (n int, err error) {
	return s.b.Read(b)
}

func (s *MockStream) AsyncRead(b []byte, cb sonic.AsyncCallback) {
	n, err := s.b.Read(b)
	cb(err, n)
}

func (s *MockStream) AsyncReadAll(b []byte, cb sonic.AsyncCallback) {
	n, err := s.b.Read(b)
	cb(err, n)
}

func (s *MockStream) Write(b []byte) (n int, err error) {
	return s.b.Write(b)
}

func (s *MockStream) AsyncWrite(b []byte, cb sonic.AsyncCallback) {
	n, err := s.b.Write(b)
	cb(err, n)
}

func (s *MockStream) AsyncWriteAll(b []byte, cb sonic.AsyncCallback) {
	n, err := s.b.Write(b)
	cb(err, n)
}

func (s *MockStream) Cancel() {

}

func (s *MockStream) AsyncClose(cb func(err error)) {

}

func (s *MockStream) Close() error {
	return nil
}

func (s *MockStream) RawFd() int {
	return -1
}
