package stream

import (
	"fmt"
	"net"
	"testing"

	"github.com/talostrading/sonic"
	"github.com/talostrading/sonic/sonicerrors"
)

type testServer struct {
	t           *testing.T
	ln          net.Listener
	acceptQueue chan struct{}
	writeQueue  chan [][]byte
	closeQueue  chan struct{}

	conn net.Conn
}

func initTestServer(
	t *testing.T,
	transport string,
) (*testServer, string) {
	s := &testServer{}

	s.t = t
	ln, err := net.Listen(transport, "localhost:0")
	if err != nil {
		s.t.Fatal(err)
	}
	s.ln = ln
	s.acceptQueue = make(chan struct{}, 1024)
	s.writeQueue = make(chan [][]byte, 1024)
	s.closeQueue = make(chan struct{}, 1024)

	go func() {
		defer s.ln.Close()
		if s.conn != nil {
			defer s.conn.Close()
		}

		for {
			select {
			case <-s.acceptQueue:
				if s.conn != nil {
					s.conn.Close()
					s.conn = nil
				}

				conn, err := s.ln.Accept()
				if err != nil {
					s.t.Error(err)
				}
				s.conn = conn
			case bs := <-s.writeQueue:
				for s.conn == nil && len(s.acceptQueue) > 0 {
				}
				for _, b := range bs {
					_, err := s.conn.Write(b) // TODO writev
					if err != nil {
						s.t.Error(err)
					}
				}
			case <-s.closeQueue:
				s.ln.Close()
				if s.conn != nil {
					s.conn.Close()
				}
				close(s.acceptQueue)
				close(s.writeQueue)
				close(s.closeQueue)
				return
			}
		}
	}()

	return s, s.ln.Addr().String()
}

func (s *testServer) accept() *testServer {
	s.acceptQueue <- struct{}{}
	return s
}

func (s *testServer) write(bs ...[]byte) *testServer {
	s.writeQueue <- bs
	return s
}

func (s *testServer) close() {
	s.closeQueue <- struct{}{}
}

func TestStream(t *testing.T) {
	ioc := sonic.MustIO()
	defer ioc.Close()

	s, addr := initTestServer(t, "tcp")
	defer s.close()
	s.accept().write([]byte("hello"))

	stream, err := Connect(ioc, "tcp", addr)
	if err != nil {
		t.Fatal(err)
	}

	var (
		n int
		b = make([]byte, 128)
	)
	n, err = stream.Read(b)
	for err == sonicerrors.ErrWouldBlock {
		n, err = stream.Read(b)
	}

	fmt.Println(n, err)
}
