package sonicwebsocket

import (
	"bufio"
	"bytes"
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
)

type testServer struct {
	conn net.Conn
}

func (s *testServer) AsyncAccept(addr string, cb func(error)) {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		cb(err)
		return
	}

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			cb(err)
			return
		}
		s.conn = conn

		b := make([]byte, 4096)
		n, err := s.conn.Read(b)
		if err != nil {
			cb(err)
			return
		}
		b = b[:n]

		req, err := http.ReadRequest(bufio.NewReader(bytes.NewBuffer(b)))
		if err != nil {
			cb(err)
			return
		}

		if !IsUpgrade(req) {
			reqb, err := httputil.DumpRequest(req, true)
			if err == nil {
				err = fmt.Errorf("request is not websocket upgrade: %s", string(reqb))
			}
			cb(err)
			return
		}

		res := bytes.NewBuffer(nil)
		res.Write([]byte("HTTP/1.1 101 Switching Protocols\r\n"))
		res.Write([]byte("Upgrade: websocket\r\n"))
		res.Write([]byte("Connection: Upgrade\r\n"))
		res.Write([]byte(fmt.Sprintf("Sec-WebSocket-Accept: %s\r\n", makeResponseKey([]byte(req.Header.Get("Sec-WebSocket-Key"))))))
		res.Write([]byte("\r\n"))

		_, err = res.WriteTo(s.conn)
		if err != nil {
			cb(err)
		}
	}()
}
