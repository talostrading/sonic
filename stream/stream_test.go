package stream

import (
	"errors"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/talostrading/sonic"
	"github.com/talostrading/sonic/sonicerrors"
)

// TODO need to see what's the current way of testing errors for equality

type testServer struct {
	t           *testing.T
	ln          net.Listener
	acceptQueue chan struct{}
	writeQueue  chan [][]byte

	conn net.Conn
	lck  sync.Mutex
}

func poll(t *testing.T, ioc *sonic.IO) {
	n, err := ioc.PollOne()
	if n == 0 && (err == nil || errors.Is(err, sonicerrors.ErrTimeout)) {
		n, err = ioc.PollOne()
	}
	if err != nil && err != sonicerrors.ErrTimeout {
		t.Fatal(err)
	}
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

	go func() {
		accept := func() {
			s.lck.Lock()
			defer s.lck.Unlock()

			if s.conn != nil {
				s.conn.Close()
				s.conn = nil
			}

			conn, err := s.ln.Accept()
			if err != nil {
				s.t.Error(err)
			}
			s.conn = conn
		}
		for range s.acceptQueue {
			accept()
		}
	}()

	go func() {
		connReady := func() bool {
			s.lck.Lock()
			defer s.lck.Unlock()
			return s.conn != nil
		}

		write := func(bs [][]byte) {
			s.lck.Lock()
			defer s.lck.Unlock()

			for _, b := range bs {
				_, err := s.conn.Write(b) // TODO writev
				if err != nil {
					s.t.Error(err)
				}
			}
		}

		for bs := range s.writeQueue {
			for !connReady() {
				time.Sleep(time.Millisecond)
			}
			write(bs)
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
	s.ln.Close()
	if s.conn != nil {
		s.conn.Close()
	}
	close(s.acceptQueue)
	close(s.writeQueue)
	return
}

func TestStreamSyncRead(t *testing.T) {
	ioc := sonic.MustIO()
	defer ioc.Close()

	s, addr := initTestServer(t, "tcp")
	defer s.close()
	s.accept().write([]byte("hello"))

	stream, err := Connect(ioc, "tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer stream.Close()

	var (
		n int
		b = make([]byte, 128)
	)
	n, err = stream.Read(b)
	for err == sonicerrors.ErrWouldBlock {
		n, err = stream.Read(b)
	}
	if err != nil {
		t.Fatal(err)
	}

	if string(b[:n]) != "hello" {
		t.Fatal("incorrect read")
	}
}

func TestStreamAsyncRead(t *testing.T) {
	ioc := sonic.MustIO()
	defer ioc.Close()

	s, addr := initTestServer(t, "tcp")
	defer s.close()
	s.accept().write([]byte("hello"))

	stream, err := Connect(ioc, "tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer stream.Close()

	b := make([]byte, 128)
	stream.AsyncRead(b, func(err error, n int) {
		if err != nil {
			t.Fatal(err)
		}

		if string(b[:n]) != "hello" {
			t.Fatal("incorrect read")
		}
	})

	poll(t, ioc)
}
