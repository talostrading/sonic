package websocket

import (
	"bufio"
	"bytes"
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"sync/atomic"

	"github.com/talostrading/sonic"
)

// MockServer is a server which can be used to test the WebSocket client.
type MockServer struct {
	ln     net.Listener
	conn   net.Conn
	closed int32
	port   int32
	portChan chan int

	Upgrade *http.Request
}

// Accept starts the mock server, listening on the specified address.
// If a callback is provided, it is invoked with the assigned port.
func (s *MockServer) Accept(addr string) (err error) {
	s.ln, err = net.Listen("tcp", addr)
	if err != nil {
		return err
	}

	port := int(s.ln.Addr().(*net.TCPAddr).Port)
	atomic.StoreInt32(&s.port, int32(port))

	if s.portChan != nil {
		s.portChan <- port
	}

	conn, err := s.ln.Accept()
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

	s.Upgrade, err = http.ReadRequest(bufio.NewReader(bytes.NewBuffer(b)))
	if err != nil {
		return err
	}

	if !IsUpgradeReq(s.Upgrade) {
		reqb, err := httputil.DumpRequest(s.Upgrade, true)
		if err == nil {
			err = fmt.Errorf(
				"request is not websocket upgrade: %s",
				string(reqb),
			)
		}
		return err
	}

	res := bytes.NewBuffer(nil)
	fmt.Fprintf(res, "HTTP/1.1 101 Switching Protocols\r\n")
	fmt.Fprintf(res, "Upgrade: websocket\r\n")
	fmt.Fprintf(res, "Connection: Upgrade\r\n")
	fmt.Fprintf(res,
		"Sec-WebSocket-Accept: %s\r\n",
		MakeResponseKey([]byte(s.Upgrade.Header.Get("Sec-WebSocket-Key"))),
	)
	fmt.Fprintf(res, "\r\n")

	_, err = res.WriteTo(s.conn)
	return err
}

func (s *MockServer) Write(b []byte) error {
	f := NewFrame()
	f.SetText()
	f.SetPayload(b)
	f.SetFIN()
	_, err := f.WriteTo(s.conn)
	return err
}

func (s *MockServer) Read(b []byte) (n int, err error) {
	f := NewFrame()

	_, err = f.ReadFrom(s.conn)
	if err == nil {
		if !f.IsMasked() {
			return 0, fmt.Errorf("client frames should be masked")
		}

		f.UnmaskPayload()
		copy(b, f.Payload())
		n = f.PayloadLength()
	}
	return n, err
}

func (s *MockServer) Close() {
	if atomic.CompareAndSwapInt32(&s.closed, 0, 1) {
		if s.conn != nil {
			_ = s.conn.Close()
		}
		if s.ln != nil {
			_ = s.ln.Close()
		}
	}
}

func (s *MockServer) IsClosed() bool {
	return atomic.LoadInt32(&s.closed) == 1
}

func (s *MockServer) Port() int {
	return int(atomic.LoadInt32(&s.port))
}

var _ sonic.Stream = &MockStream{}

// MockStream is a mock TCP stream that's not attached to any operating system
// IO executor. It is used to test reads and writes for WebSocket servers and
// clients.
//
// A WebsocketStream can be set to use a MockStream only if it is in
// StateActive, which occurs after a successful handshake or a call to init().
type MockStream struct {
	b *sonic.ByteBuffer
}

func NewMockStream() *MockStream {
	s := &MockStream{
		b: sonic.NewByteBuffer(),
	}
	return s
}

func (s *MockStream) Read(b []byte) (n int, err error) {
	return s.b.Read(b)
}

func (s *MockStream) AsyncRead(b []byte, cb sonic.AsyncCallback) {
	n, err := s.b.Read(b)
	cb(err, n)
}

func (s *MockStream) AsyncReadAll(b []byte, cb sonic.AsyncCallback) {
	n, err := s.b.Read(b)
	cb(err, n)
}

func (s *MockStream) Write(b []byte) (n int, err error) {
	return s.b.Write(b)
}

func (s *MockStream) AsyncWrite(b []byte, cb sonic.AsyncCallback) {
	n, err := s.b.Write(b)
	cb(err, n)
}

func (s *MockStream) AsyncWriteAll(b []byte, cb sonic.AsyncCallback) {
	n, err := s.b.Write(b)
	cb(err, n)
}

func (s *MockStream) Cancel() {

}

func (s *MockStream) AsyncClose(cb func(err error)) {

}

func (s *MockStream) Close() error {
	return nil
}

func (s *MockStream) RawFd() int {
	return -1
}

// GetFreePort asks the kernel for a free open port that is ready to use.
// From: https://gist.github.com/sevkin/96bdae9274465b2d09191384f86ef39d
func GetFreePort() (port int, err error) {
	var a *net.TCPAddr
	if a, err = net.ResolveTCPAddr("tcp", "localhost:0"); err == nil {
		var l *net.TCPListener
		if l, err = net.ListenTCP("tcp", a); err == nil {
			defer l.Close()
			return l.Addr().(*net.TCPAddr).Port, nil
		}
	}
	return
}
