package http

import (
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"syscall"
	"time"

	"github.com/talostrading/sonic"
)

var (
	_ Stream = &HttpStream{}

	MaxRetries  = 5
	DialTimeout = 5 * time.Second
)

type HttpStream struct {
	ioc   *sonic.IO
	tls   *tls.Config
	role  Role
	state StreamState

	url *url.URL

	conn   net.Conn
	stream sonic.Stream

	retries int

	dialer *net.Dialer

	ccs *sonic.BlockingCodecStream[*http.Request, *http.Response]
	scs *sonic.BlockingCodecStream[*http.Response, *http.Request]

	src *sonic.ByteBuffer
	dst *sonic.ByteBuffer
}

func NewHttpStream(ioc *sonic.IO, tls *tls.Config, role Role) (*HttpStream, error) {
	if role != RoleClient {
		return nil, fmt.Errorf("unsupported role=%s", role)
	}

	s := &HttpStream{
		ioc:   ioc,
		tls:   tls,
		role:  role,
		state: StateDisconnected,
		src:   sonic.NewByteBuffer(),
		dst:   sonic.NewByteBuffer(),
		dialer: &net.Dialer{
			Timeout: DialTimeout,
		},
	}
	s.src.Reserve(16 * 1024)
	s.dst.Reserve(16 * 1024)

	return s, nil
}

func (s *HttpStream) Proto() string {
	return "HTTP/1.1"
}

func (s *HttpStream) Connect(addr string) (err error) {
	if s.role != RoleClient {
		return fmt.Errorf("can only connect in a client role")
	}

	s.url, err = url.Parse(addr)
	if err != nil {
		return err
	}

	return s.connect()
}

func (s *HttpStream) connect() (err error) {
	done := make(chan struct{}, 1)
	s.dial(func(uerr error) {
		done <- struct{}{}
		err = uerr
	})
	<-done
	return
}

func (s *HttpStream) AsyncConnect(addr string, cb func(err error)) {
	if s.role != RoleClient {
		cb(fmt.Errorf("can only connect in a client role"))
		return
	}

	var err error
	s.url, err = url.Parse(addr)
	if err != nil {
		cb(err)
		return
	}

	s.asyncConnect(cb)
}

func (s *HttpStream) asyncConnect(cb func(err error)) {
	go func() {
		s.dial(func(err error) {
			s.ioc.Post(func() {
				cb(err)
			})
		})
	}()
}

func (s *HttpStream) dial(cb func(err error)) {
	var (
		err error
		sc  syscall.Conn
	)

	port := s.url.Port()

	switch s.url.Scheme {
	case "http":
		if port == "" {
			port = "80"
		}

		s.conn, err = net.DialTimeout("tcp", s.url.Hostname()+":"+port, DialTimeout)
		if err == nil {
			sc = s.conn.(syscall.Conn)
		}
	case "https":
		if s.tls == nil {
			cb(fmt.Errorf("TLS config required for https addresses"))
			return
		}

		if port == "" {
			port = "443"
		}

		s.conn, err = tls.DialWithDialer(s.dialer, "tcp", s.url.Hostname()+":"+port, s.tls)
		if err == nil {
			sc = s.conn.(*tls.Conn).NetConn().(syscall.Conn)
		}
	default:
		cb(fmt.Errorf("invalid scheme %s", s.url.Scheme))
		return
	}

	if err == nil {
		sonic.NewAsyncAdapter(s.ioc, sc, s.conn, func(err error, stream *sonic.AsyncAdapter) {
			if err == nil {
				err = s.init(stream)
			}
			cb(err)
		})
	} else {
		cb(err)
	}
}

func (s *HttpStream) init(stream sonic.Stream) (err error) {
	s.stream = stream

	switch s.role {
	case RoleClient:
		codec := NewClientCodec(s.src, s.dst)
		s.ccs, err = sonic.NewBlockingCodecStream[*http.Request, *http.Response](stream, codec, s.src, s.dst)
	case RoleServer:
		codec := NewServerCodec(s.src, s.dst)
		s.scs, err = sonic.NewBlockingCodecStream[*http.Response, *http.Request](stream, codec, s.src, s.dst)
	default:
		err = fmt.Errorf("unknown role")
	}

	if err == nil {
		s.state = StateConnected
	}

	return
}

func (s *HttpStream) prepareRequest(target string, req *http.Request) {
	req.ProtoMinor = 1
	req.ProtoMajor = 1
	req.URL = s.url
	req.Host = s.url.Host

	if i := strings.Index(target, "?"); i >= 0 {
		req.URL.RawQuery = target[i+1:]
		req.URL.Path = target[:i]
	} else {
		req.URL.Path = target
	}

	fmt.Println(req.URL)
}

func (s *HttpStream) Do(target string, req *http.Request) (*http.Response, error) {
	if s.role != RoleClient {
		return nil, fmt.Errorf("can only send request in the Client role")
	}

	s.prepareRequest(target, req)
	return s.do(req)
}

func (s *HttpStream) do(req *http.Request) (res *http.Response, err error) {
	// https://www.rfc-editor.org/rfc/rfc2616#section-14.10
	if req.Close {
		s.state = StateDisconnecting
		defer func() {
			s.Close()
		}()
	}

	for err == nil {
		_, err = s.ccs.WriteNext(req)
		if err == io.EOF {
			if s.retries < MaxRetries {
				s.retries++
				err = s.reconnect()
			} else {
				s.state = StateDisconnected
			}
		}

		if err == nil {
			res, err = s.ccs.ReadNext()
			if err == io.EOF {
				if s.retries < MaxRetries {
					s.retries++
					err = s.reconnect()
				}
			} else {
				s.state = StateDisconnected
			}
		}
	}

	s.retries = 0

	if res != nil && res.Close {
		s.state = StateDisconnected
	}

	return
}

func (s *HttpStream) reconnect() error {
	prev := s.state // can be StateConnected or StateDisconnecting

	s.state = StateReconnecting
	err := s.connect()
	if err != nil {
		s.state = StateDisconnected
	} else {
		s.state = prev
	}

	return err
}

func (s *HttpStream) AsyncDo(target string, req *http.Request, cb AsyncResponseHandler) {
	if s.role != RoleClient {
		cb(fmt.Errorf("can only send request in the Client role"), nil)
		return
	}
	s.prepareRequest(target, req)
	s.asyncDo(req, cb)
}

func (s *HttpStream) asyncDo(req *http.Request, cb AsyncResponseHandler) {
	// https://www.rfc-editor.org/rfc/rfc2616#section-14.10
	if req.Close {
		s.state = StateDisconnecting
	}

	s.asyncTryWriteRequest(req, func(err error) {
		if err == nil {
			s.asyncTryReadResponse(req, func(err error, res *http.Response) {
				if req.Close {
					s.Close()
				}

				if res != nil && res.Close {
					s.state = StateDisconnected
				}

				s.retries = 0

				cb(err, res)
			})
		} else {
			s.retries = 0

			cb(err, nil)
		}
	})
}

func (s *HttpStream) asyncTryWriteRequest(req *http.Request, cb func(err error)) {
	s.ccs.AsyncWriteNext(req, func(err error, _ int) {
		if err == io.EOF {
			if s.retries < MaxRetries {
				s.retries++
				s.asyncReconnect(func(err error) {
					if err != nil {
						cb(err)
					} else {
						s.asyncTryWriteRequest(req, cb)
					}
				})
			} else {
				s.state = StateDisconnected
				cb(err)
			}
		} else {
			cb(err)
		}
	})
}

func (s *HttpStream) asyncTryReadResponse(req *http.Request, cb func(err error, res *http.Response)) {
	s.ccs.AsyncReadNext(func(err error, res *http.Response) {
		if err == io.EOF {
			if s.retries < MaxRetries {
				s.retries++
				s.asyncReconnect(func(err error) {
					if err != nil {
						cb(err, res)
					} else {
						s.asyncDo(req, cb)
					}
				})
			} else {
				s.state = StateDisconnected
				cb(err, res)
			}
		} else {
			cb(err, res)
		}
	})
}

func (s *HttpStream) asyncReconnect(cb func(err error)) {
	prev := s.state // can be StateConnected or StateDisconnecting

	s.state = StateReconnecting
	s.asyncConnect(func(err error) {
		if err != nil {
			s.state = StateDisconnected
		} else {
			s.state = prev
		}

		cb(err)
	})
}

func (s *HttpStream) NextLayer() sonic.Stream {
	return s.stream
}

func (s *HttpStream) State() StreamState {
	return s.state
}

func (s *HttpStream) Close() {
	if s.state != StateDisconnected {
		s.url = nil
		s.state = StateDisconnected
		s.stream.Close()
	}
}
