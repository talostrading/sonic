package sonicwebsocket

import (
	"bufio"
	"bytes"
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"

	"github.com/talostrading/sonic"
)

type testServer struct {
	conn net.Conn
}

func (s *testServer) Accept(addr string) error {
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
	return err
}

func (s *testServer) Write(fr *Frame) (n int, err error) {
	var nn int64
	nn, err = fr.WriteTo(s.conn)
	n = int(nn)
	return
}

func (s *testServer) Read(b []byte) (n int, err error) {
	frame := AcquireFrame()
	defer ReleaseFrame(frame)

	nn, err := frame.ReadFrom(s.conn)
	if err == nil {
		copy(b, frame.Payload())
	}

	return int(nn), err
}

func (s *testServer) Close() {
	s.conn.Close()
}

type testClient struct {
	ws Stream
}

func AsyncNewTestClient(ioc *sonic.IO, addr string, cb func(err error, cl *testClient)) {
	cl := &testClient{}
	var err error
	cl.ws, err = NewWebsocketStream(ioc, nil, RoleClient)

	if err != nil {
		cb(err, nil)
	} else {
		cl.ws.AsyncHandshake(addr, func(err error) {
			cb(err, cl)
		})
	}
}

func (c *testClient) RunReadLoop(b []byte, cb AsyncCallback) {
	c.asyncRead(b, cb)
}

func (c *testClient) asyncRead(b []byte, cb AsyncCallback) {
	b = b[:cap(b)]
	c.ws.AsyncRead(b, func(err error, n int, mt MessageType) {
		cb(err, n, mt)

		if err == nil {
			c.asyncRead(b, cb)
		}
	})
}
