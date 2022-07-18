package websocket

import (
	"bufio"
	"bytes"
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"sync"

	"github.com/talostrading/sonic"
)

// MockServer is a server which can be used to test the WebSocket client.
type MockServer struct {
	conn   net.Conn
	lck    sync.Mutex // so it can be used concurrently
	closed bool
}

func (s *MockServer) Accept(addr string) error {
	s.lck.Lock()
	defer s.lck.Unlock()

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}

	conn, err := ln.Accept()
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
	res.Write([]byte(fmt.Sprintf("Sec-WebSocket-Accept: %s\r\n", makeResponseKey([]byte(req.Header.Get("Sec-WebSocket-Key"))))))
	res.Write([]byte("\r\n"))

	_, err = res.WriteTo(s.conn)

	return nil
}
func (s *MockServer) Write(fr *Frame) (n int, err error) {
	s.lck.Lock()
	defer s.lck.Unlock()

	var nn int64
	nn, err = fr.WriteTo(s.conn)
	n = int(nn)
	return
}

func (s *MockServer) Read(fr *Frame) (n int, err error) {
	s.lck.Lock()
	defer s.lck.Unlock()

	nn, err := fr.ReadFrom(s.conn)
	return int(nn), err
}

func (s *MockServer) Close() {
	s.lck.Lock()
	defer s.lck.Unlock()

	if s.closed {
		return
	}
	s.closed = true

	s.conn.Close()
}

func (s *MockServer) IsClosed() bool {
	return s.closed
}

var _ sonic.Stream = &MockStream{}

// MockStream is a stream that's not attached to any operating system IO object.
// It is used to test WebSocket servers and clients.
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
