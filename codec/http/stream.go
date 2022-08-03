package http

import (
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"syscall"
	"time"

	"github.com/talostrading/sonic"
)

var (
	_ Stream = &HttpStream{}

	MaxRetries  = 5
	DialTimeout = 5 * time.Second
)

// TODO https://www.rfc-editor.org/rfc/rfc2616#section-14.10

type HttpStream struct {
	ioc   *sonic.IO
	tls   *tls.Config
	role  Role
	state StreamState

	addr string
	url  *url.URL

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

	done := make(chan struct{}, 1)
	s.dial(addr, func(uerr error) {
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

	go func() {
		s.dial(addr, func(err error) {
			s.ioc.Post(func() {
				cb(err)
			})
		})
	}()
}

func (s *HttpStream) dial(addr string, cb func(err error)) {
	var (
		err error
		sc  syscall.Conn
	)

	s.addr = addr
	s.url, err = url.Parse(addr)
	if err != nil {
		cb(err)
		return
	}

	switch s.url.Scheme {
	case "http":
		s.conn, err = net.DialTimeout("tcp", s.url.Host, DialTimeout)
		if err == nil {
			sc = s.conn.(syscall.Conn)
		}
	case "https":
		if s.tls == nil {
			cb(fmt.Errorf("TLS config required for https addresses"))
			return
		}

		s.conn, err = tls.DialWithDialer(s.dialer, "tcp", s.url.Host, s.tls)
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

		s.state = StateConnected
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

func (s *HttpStream) prepareRequest(req *http.Request) {
	req.ProtoMinor = 1
	req.ProtoMajor = 1
	req.URL = s.url
	req.Host = s.url.Host
}

func (s *HttpStream) Do(req *http.Request) (*http.Response, error) {
	if s.role != RoleClient {
		return nil, fmt.Errorf("can only send request in the Client role")
	}

	s.prepareRequest(req)
	return s.do(req)
}

func (s *HttpStream) do(req *http.Request) (res *http.Response, err error) {
	for s.retries <= MaxRetries {
		_, err = s.ccs.WriteNext(req)
		if err == io.EOF {
			s.retries++
			continue
		} else if err != nil {
			return nil, err
		}

		res, err = s.ccs.ReadNext()
		if err == io.EOF {
			s.retries++
			continue
		} else {
			return res, err
		}
	}

	return
}

func (s *HttpStream) AsyncDo(req *http.Request, cb AsyncResponseHandler) {
	if s.role != RoleClient {
		cb(fmt.Errorf("can only send request in the Client role"), nil)
		return
	}
	s.prepareRequest(req)
	s.asyncDo(req, cb)
}

func (s *HttpStream) asyncDo(req *http.Request, cb AsyncResponseHandler) {
	s.ccs.AsyncWriteNext(req, func(err error, _ int) {
		if err == io.EOF {
			s.state = StateDisconnected

			if s.retries < MaxRetries {
				s.asyncRetry(req, cb)
			} else {
				cb(err, nil)
			}
		} else if err != nil {
			cb(err, nil)
		} else {
			s.asyncReadResponse(req, cb)
		}
	})
}

func (s *HttpStream) asyncReadResponse(req *http.Request, cb AsyncResponseHandler) {
	s.ccs.AsyncReadNext(func(err error, res *http.Response) {
		if err == io.EOF {
			s.state = StateDisconnected

			if s.retries < MaxRetries {
				s.asyncRetry(req, cb)
			} else {
				cb(err, nil)
			}
		} else {
			s.retries = 0
			cb(err, res)
		}
	})
}

func (s *HttpStream) asyncRetry(req *http.Request, cb AsyncResponseHandler) {
	s.retries++

	s.AsyncConnect(s.addr, func(err error) {
		if err != nil {
			cb(err, nil)
		} else {
			s.asyncDo(req, cb)
		}
	})
}

func (s *HttpStream) Close() {
	if s.state == StateConnected {
		s.state = StateDisconnected
		s.stream.Close()
	}
}

func (s *HttpStream) State() StreamState {
	return s.state
}
