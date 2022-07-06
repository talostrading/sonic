package sonicwebsocket

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"syscall"

	"github.com/talostrading/sonic"
)

func (s *WebsocketStream) Handshake(addr string) (err error) {
	if s.role != RoleClient {
		return fmt.Errorf("invalid role=%s; only clients can handshake", s.role)
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
		cb(fmt.Errorf("invalid role=%s; only clients can establis", s.role))
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

func (s *WebsocketStream) handshake(addr string, cb func(error)) {
	s.state = StateHandshake

	uri, err := s.resolveAddr(addr)
	if err != nil {
		cb(err)
		s.state = StateClosed
	} else {
		s.dial(uri, func(err error) {
			if err != nil {
				cb(err)
				s.state = StateClosed
			} else {
				err = s.upgrade(uri)
				if err != nil {
					s.state = StateClosed
				} else {
					s.state = StateOpen
				}
				cb(err)
			}
		})
	}
}

func (s *WebsocketStream) resolveAddr(addr string) (*url.URL, error) {
	url, err := url.Parse(addr)
	if err != nil {
		return nil, err
	}

	switch url.Scheme {
	case "ws":
		url.Scheme = "http"
	case "wss":
		url.Scheme = "https"
	default:
		return nil, fmt.Errorf("invalid address %s", addr)
	}

	return url, nil
}

func (s *WebsocketStream) dial(uri *url.URL, cb func(err error)) {
	var conn net.Conn
	var sc syscall.Conn
	var err error

	if uri.Scheme == "http" {
		port := uri.Port()
		if port == "" {
			port = "80"
		}

		addr := uri.Hostname() + ":" + port
		conn, err = net.Dial("tcp", addr)
		if err != nil {
			cb(err)
			return
		}
		sc = conn.(syscall.Conn)
	} else {
		if s.tls == nil {
			cb(fmt.Errorf("wss requested with nil tls config"))
			return
		}
		port := uri.Port()
		if port == "" {
			port = "443"
		}

		addr := uri.Hostname() + ":" + port
		conn, err = tls.Dial("tcp", addr, s.tls) // TODO dial timeout
		if err != nil {
			cb(err)
			return
		}
		sc = conn.(*tls.Conn).NetConn().(syscall.Conn)
	}

	sonic.NewAsyncAdapter(s.ioc, sc, conn, func(err error, stream *sonic.AsyncAdapter) {
		if err != nil {
			cb(err)
		} else {
			s.stream = stream
			cb(nil)
		}
	})
}

func (s *WebsocketStream) upgrade(uri *url.URL) error {
	req, err := http.NewRequest("GET", uri.String(), nil)
	if err != nil {
		return err
	}

	sentKey, expectedKey := s.makeKey()
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Connection", "upgrade")
	req.Header.Set("Sec-WebSocket-Key", string(sentKey))
	req.Header.Set("Sec-Websocket-Version", "13")

	err = req.Write(s.stream)
	if err != nil {
		return err
	}

	n, err := s.stream.Read(s.rbuf)
	if err != nil {
		return err
	}
	s.rbuf = s.rbuf[:n]
	rd := bytes.NewReader(s.rbuf)
	res, err := http.ReadResponse(bufio.NewReader(rd), req)
	if err != nil {
		return err
	}

	rawRes, err := httputil.DumpResponse(res, true)
	if err != nil {
		return err
	}

	resLen := len(rawRes)
	extra := len(s.rbuf) - resLen
	if extra > 0 {
		s.rbuf = s.rbuf[resLen:]
	} else {
		s.rbuf = s.rbuf[:0]
	}

	if !(res.StatusCode == 101 && strings.EqualFold(res.Header.Get("Upgrade"), "websocket")) {
		// TODO somehow wrap errors here so you can attach a message
		return ErrCannotUpgrade
	}

	if key := res.Header.Get("Sec-WebSocket-Accept"); key != expectedKey {
		// TODO somehow wrap errors here so you can attach a message
		return ErrCannotUpgrade
	}

	return nil
}

// makeKey generates the key of Sec-WebSocket-Key header as well as the expected
// response present in Sec-WebSocket-Accept header.
func (s *WebsocketStream) makeKey() (req, res string) {
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
