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

// Write writes the buffer as a message composed of `nframe` frames.
//
// Note: The continuation opcode is 0.
//
// An unfragmented message consists of a single frame with the FIN bit set and an opcode other than 0.
//
// A fragmented message consists of a single frame with the FIN bit clear and an opcode other than 0,
// followed by zero or more frames with the FIN bit clear and the opcode set to 0, and terminated by
// a single frame with the FIN bit set and an opcode of 0.
func (s *testServer) Write(b []byte, nframe int, text bool) (n int, err error) {
	nchunk := len(b) / nframe
	for i := 0; i < nframe; i++ {
		start := i * nchunk
		end := (i + 1) * nchunk
		if i == nframe-1 {
			end = len(b)
		}
		chunk := b[start:end]

		frame := AcquireFrame()
		if i == 0 {
			if text {
				frame.SetText()
			} else {
				frame.SetBinary()
			}
		} else if i == nframe-1 {
			frame.SetContinuation()
		} else {
			frame.SetContinuation()
			frame.SetFin()
		}
		frame.SetPayload(chunk)

		nn, err := frame.WriteTo(s.conn)
		n += int(nn)
		if err != nil {
			return n, err
		}

		ReleaseFrame(frame)
	}

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
	// TODO proper closing handshake
	s.conn.Close()
}
