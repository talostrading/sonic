package main

import (
	"fmt"

	"github.com/talostrading/sonic"
	"github.com/talostrading/sonic/sonicopts"
)

func main() {
	ioc := sonic.MustIO()

	s, err := NewServer(ioc)
	if err != nil {
		panic(err)
	}

	s.Run()

	for {
		ioc.RunOne()
	}
}

type Server struct {
	ln       sonic.Listener
	handlers []*Handler
}

func NewServer(ioc *sonic.IO) (*Server, error) {
	ln, err := sonic.Listen(ioc, "tcp", ":8080", sonicopts.Nonblocking(true))
	if err != nil {
		return nil, err
	}
	s := &Server{
		ln: ln,
	}
	return s, nil
}

func (s *Server) Run() {
	s.asyncAccept()
}

func (s *Server) asyncAccept() {
	s.ln.AsyncAccept(s.onAsyncAccept)
}

func (s *Server) onAsyncAccept(err error, conn sonic.Conn) {
	if err != nil {
		panic(err)
	} else {
		handler := NewHandler(conn)
		handler.Run()
		s.handlers = append(s.handlers, handler)
		fmt.Println("accepted conn", len(s.handlers))
		s.asyncAccept()
	}
}

type Handler struct {
	conn sonic.Conn
}

func NewHandler(conn sonic.Conn) *Handler {
	h := &Handler{
		conn: conn,
	}
	return h
}

func (h *Handler) Run() {
	h.asyncWrite()
}

var hello = []byte("hello")

func (h *Handler) asyncWrite() {
	h.conn.AsyncWrite(hello, h.onAsyncWrite)
}

func (h *Handler) onAsyncWrite(err error, n int) {
	if err != nil {
		h.conn.Close()

		if err != sonic.ErrEOF {
			fmt.Println("error on write")
			panic(err)
		}
		fmt.Println("conn closed")
		return
	}

	h.asyncWrite()
}
