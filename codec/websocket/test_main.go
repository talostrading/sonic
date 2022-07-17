package websocket

import (
	"bufio"
	"bytes"
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"sync"
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
