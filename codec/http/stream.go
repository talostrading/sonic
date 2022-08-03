package http

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"syscall"

	"github.com/talostrading/sonic"
)

var _ Stream = &HttpStream{}

// TODO https://www.rfc-editor.org/rfc/rfc2616#section-14.10

type HttpStream struct {
	ioc   *sonic.IO
	tls   *tls.Config
	role  Role
	state StreamState

	conn   net.Conn
	stream sonic.Stream

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
	}
	s.src.Reserve(16 * 1024)
	s.dst.Reserve(16 * 1024)

	return s, nil
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

	if s.tls == nil {
		s.conn, err = net.Dial("tcp", addr)
		if err == nil {
			sc = s.conn.(syscall.Conn)
		}
	} else {
		s.conn, err = tls.Dial("tcp", addr, s.tls)
		if err == nil {
			sc = s.conn.(*tls.Conn).NetConn().(syscall.Conn)
		}
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

func (s *HttpStream) Do(req *http.Request) (*http.Response, error) {
	if s.role != RoleClient {
		return nil, fmt.Errorf("can only send request in the Client role")
	}

	_, err := s.ccs.WriteNext(req)
	if err != nil {
		return nil, err
	}

	return s.ccs.ReadNext()
}

func (s *HttpStream) AsyncDo(req *http.Request, cb AsyncResponseHandler) {
	if s.role != RoleClient {
		cb(fmt.Errorf("can only send request in the Client role"), nil)
		return
	}

	s.ccs.AsyncWriteNext(req, func(err error, _ int) {
		if err != nil {
			cb(err, nil)
		} else {
			s.ccs.AsyncReadNext(cb)
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
