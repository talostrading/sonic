package main

import (
	"fmt"
	"time"

	"github.com/talostrading/sonic"
	"github.com/talostrading/sonic/sonicopts"
)

var buf = make([]byte, 16384)

type Server struct {
	ln  sonic.Listener
	ioc *sonic.IO
}

func NewServer(ioc *sonic.IO, network, addr string) (*Server, error) {
	ln, err := sonic.Listen(ioc, network, addr, sonicopts.Nonblocking(true))
	if err != nil {
		return nil, err
	}

	s := &Server{
		ln:  ln,
		ioc: ioc,
	}

	return s, nil
}

func (s *Server) Listen() {
	s.ln.AsyncAccept(s.onAccept)
}

func (s *Server) onAccept(err error, conn sonic.Conn) {
	if err != nil {
		panic(err)
	}

	s.onConn(conn)

	s.Listen()
}

func (s *Server) onConn(conn sonic.Conn) {
	conn.AsyncRead(buf, func(err error, n int) {
		s.onConnRead(conn, err, n)
	})
}

func (s *Server) onConnRead(conn sonic.Conn, err error, n int) {
	if err != nil {
		panic(err)
	}
	fmt.Println(time.Now(), err, n)
	s.onConn(conn)
}

func main() {
	ioc := sonic.MustIO(-1)

	s, err := NewServer(ioc, "tcp", ":5001")
	if err != nil {
		panic(err)
	}

	s.Listen()

	for {
		ioc.RunOne(0)
	}
}
