package frame

import (
	"encoding/binary"
	"fmt"
	"log"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/talostrading/sonic"
)

type testServer struct {
	port int
	ln   net.Listener
	n    int32
}

func (s *testServer) written() int {
	return int(atomic.LoadInt32(&s.n))
}

func setupServer() (*testServer, error) {
	ln, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		return nil, err
	}

	port := ln.Addr().(*net.TCPAddr).Port

	return &testServer{port: port, ln: ln}, nil
}

func (s *testServer) run(rate time.Duration) {
	conn, err := s.ln.Accept()
	if err != nil {
		panic(err)
	}

	payload := []byte("hello, world!")
	b := make([]byte, HeaderLen+len(payload))
	binary.BigEndian.PutUint32(b[:HeaderLen], uint32(len(payload)))
	copy(b[HeaderLen:], payload)

	write := func() error {
		written := 0
		for {
			n, err := conn.Write(b)
			if err != nil {
				return err
			}
			written += n
			if written >= HeaderLen+len(payload) {
				break
			}
		}
		return nil
	}

	for {
		err := write()
		if err != nil {
			break
		}

		atomic.AddInt32(&s.n, 1)

		if rate > 0 {
			time.Sleep(rate)
		}
	}
}

func (s *testServer) close() {
	s.ln.Close()
}

func runClient(port int, t *testing.T) {
	ioc := sonic.MustIO()
	defer ioc.Close()

	conn, err := sonic.Dial(
		ioc,
		"tcp",
		fmt.Sprintf("localhost:%d", port),
	)
	if err != nil {
		t.Fatal(err)
	}

	src := sonic.NewByteBuffer()
	dst := sonic.NewByteBuffer()
	codec := NewCodec(src)
	codecConn, err := sonic.NewNonblockingCodecConn[[]byte, []byte](
		conn, codec, src, dst,
	)
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	var fn func(error, []byte)
	fn = func(err error, frame []byte) {
		if err != nil {
			t.Fatal(err)
		} else {
			if string(frame) != "hello, world!" {
				t.Fatalf("invalid frame on read %d", n)
			}
			n++
			codecConn.AsyncReadNext(fn)
		}
	}
	codecConn.AsyncReadNext(fn)

	start := time.Now()
	for time.Since(start).Seconds() < 5 {
		ioc.PollOne()
	}

	if n == 0 {
		t.Fatal("did not read anything")
	}
	log.Printf("read %d frames", n)
}

func TestStreamRate0(t *testing.T) {
	server, err := setupServer()
	if err != nil {
		t.Fatal(err)
	}
	defer server.close()

	go func() {
		server.run(0)
	}()

	runClient(server.port, t)
	log.Printf("server wrote %d frames", server.written())
}

func TestStreamRate1us(t *testing.T) {
	server, err := setupServer()
	if err != nil {
		t.Fatal(err)
	}
	defer server.close()

	go func() {
		server.run(time.Microsecond)
	}()

	runClient(server.port, t)
	log.Printf("server wrote %d frames", server.written())
}

func TestStreamRate1ms(t *testing.T) {
	server, err := setupServer()
	if err != nil {
		t.Fatal(err)
	}
	defer server.close()

	go func() {
		server.run(time.Millisecond)
	}()

	runClient(server.port, t)
	log.Printf("server wrote %d frames", server.written())
}

func TestStreamRate1s(t *testing.T) {
	server, err := setupServer()
	if err != nil {
		t.Fatal(err)
	}
	defer server.close()

	go func() {
		server.run(time.Second)
	}()

	runClient(server.port, t)
	log.Printf("server wrote %d frames", server.written())
}
