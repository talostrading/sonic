package main

import (
	"fmt"
	"io"

	"github.com/talostrading/sonic"
	"github.com/talostrading/sonic/sonicopts"
)

func main() {
	ioc := sonic.MustIO(-1)

	s, err := NewServer(ioc, "tcp", ":5001")
	if err != nil {
		panic(err)
	}

	s.Run()

	for {
		ioc.RunOne(0)
	}
}

type ConnSink interface {
	OnConnError(id int, err error)
	OnConnClose(id int)
}

var _ ConnSink = &Server{}

var connID = 0

func nextConnID() int {
	connID++
	return connID
}

type Server struct {
	ln    sonic.Listener
	ioc   *sonic.IO
	conns map[int]*Conn
}

func NewServer(ioc *sonic.IO, network, addr string) (*Server, error) {
	ln, err := sonic.Listen(ioc, network, addr, sonicopts.Nonblocking(true))
	if err != nil {
		return nil, err
	}

	s := &Server{
		ln:    ln,
		ioc:   ioc,
		conns: make(map[int]*Conn),
	}

	return s, nil
}

func (s *Server) Run() {
	s.asyncAccept()
}

func (s *Server) asyncAccept() {
	s.ln.AsyncAccept(s.onAccept)
}

func (s *Server) onAccept(err error, sc sonic.Conn) {
	if err != nil {
		panic(err)
	}

	id := nextConnID()
	conn := NewConn(id, sc, s)
	s.conns[id] = conn
	conn.Run()

	fmt.Printf("conn %d accepted\n", id)

	s.asyncAccept()
}

func (s *Server) OnConnError(id int, err error) {
	fmt.Printf("conn %d error=%v\n", id, err)
	delete(s.conns, id)
}

func (s *Server) OnConnClose(id int) {
	fmt.Printf("conn %d closed", id)
	delete(s.conns, id)
}

type Conn struct {
	conn sonic.Conn
	buf  []byte
	sink ConnSink
	id   int
}

func NewConn(id int, conn sonic.Conn, sink ConnSink) *Conn {
	c := &Conn{
		id:   id,
		conn: conn,
		buf:  make([]byte, 4096),
		sink: sink,
	}
	return c
}

func (c *Conn) Run() {
	c.asyncRead()
}

func (c *Conn) asyncRead() {
	c.conn.AsyncRead(c.buf, c.onAsyncRead)
}

func (c *Conn) onAsyncRead(err error, n int) {
	if err != nil {
		fmt.Println("conn: error", err)
		if err == io.EOF {
			c.conn.Close()
			c.sink.OnConnClose(c.id)
		} else {
			c.sink.OnConnError(c.id, err)
		}
	} else {
		c.asyncRead()
	}
}
