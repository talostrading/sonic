package sonic

import (
	"fmt"
	"io"
	"net"
	"sync"
	"testing"
)

func TestDequeueWithTenFiles(t *testing.T) {
	serv := newServer("tcp", ":8080", 10)
	serv.start()
	conns, _, queue := createDefaultTestEnv()
	for i, c := range conns {
		c.AsyncWrite([]byte(fmt.Sprintf("%v", i)), func(error, int) {
			c.Close()
		})
	}
	queue.Dequeue(conns[1].(*qConn).file)
	if queue.pending[1] != true {
		t.Fatal("file 1 should be pending")
	}
	queue.Dequeue(conns[0].(*qConn).file)
	if len(queue.order) != 8 && len(queue.pending) != 8 {
		t.Fatal("queue should have 8 items")
	}
	queue.Dequeue(conns[2].(*qConn).file)
	if len(queue.order) != 7 && len(queue.pending) != 7 {
		t.Fatal("queue should have 7 items")
	}
	queue.Dequeue(conns[3].(*qConn).file)
	if len(queue.order) != 6 && len(queue.pending) != 6 {
		t.Fatal("queue should have 6 items")
	}
	queue.Dequeue(conns[6].(*qConn).file)
	if queue.pending[2] != true && len(queue.order) != 6 && len(queue.pending) != 6 {
		t.Fatal("file 6 should be pending and queue should have 6 items")
	}
	queue.Dequeue(conns[5].(*qConn).file)
	if queue.pending[1] != true && queue.pending[2] != true && len(queue.order) != 6 && len(queue.pending) != 6 {
		t.Fatal("file 5 and 6 should be pending and queue should have 5 items")
	}
	queue.Dequeue(conns[4].(*qConn).file)
	if len(queue.order) != 3 && len(queue.pending) != 3 {
		t.Fatal("queue should have 3 items")
	}
	queue.Dequeue(conns[8].(*qConn).file)
	if queue.pending[1] != true && len(queue.order) != 3 && len(queue.pending) != 3 {
		t.Fatal("file 8 should be pending and queue should have 3 items")
	}
	queue.Dequeue(conns[7].(*qConn).file)
	if len(queue.order) != 1 && len(queue.pending) != 1 {
		t.Fatal("file 8 should be pending and queue should have 3 items")
	}
	queue.Dequeue(conns[9].(*qConn).file)
	if len(queue.order) != 0 && len(queue.pending) != 0 {
		t.Fatal("que should have 0 items")
	}
}

func TestDeregister(t *testing.T) {
	//TODO: shiakas
}

func Test(t *testing.T) {
	serv := newServer("tcp", ":8080", 10)
	serv.start()
	conns, iocRunner, _ := createDefaultTestEnv()
	for i, c := range conns {
		c.AsyncWrite([]byte(fmt.Sprintf("%v", i)), func(error, int) {
			c.Close()
		})
	}
	iocRunner()
	serv.wg.Wait()
	for _, b := range serv.b {
		fmt.Println(string(b))
	}
}

func createDefaultTestEnv() ([]Conn, func() error, *Queue) {
	return createTestEnv(10, "tcp", "localhost:8080")
}

func createTestEnv(numberOfConnections int, connType, port string) ([]Conn, func() error, *Queue) {
	ioc := MustIO()
	queue := NewQueue()
	conns := make([]Conn, numberOfConnections)
	for i := 0; i < numberOfConnections; i++ {
		conns[i], _ = QDial(ioc, connType, port, queue)
	}
	runner := func() error {
		err := ioc.RunPending()
		defer ioc.Close()
		if err != nil {
			return err
		}
		return nil
	}
	return conns, runner, queue
}

type server struct {
	b             [][]byte
	writeChan     chan []byte
	wg            sync.WaitGroup
	connType      string
	port          string
	expectedConns int
}

func newServer(connType string, port string, connNumber int) *server {
	return &server{
		b:             make([][]byte, 0),
		writeChan:     make(chan []byte),
		connType:      connType,
		port:          port,
		expectedConns: connNumber,
	}
}

func (s *server) start() {
	ln, err := net.Listen(s.connType, s.port)
	if err != nil {
		panic(err)
	}

	s.wg.Add(1)
	go s.accept(ln)

	go func() {
		s.wg.Wait()
		close(s.writeChan)
	}()

	go func() {
		for b := range s.writeChan {
			s.b = append(s.b, b)
		}
	}()
}

func (s *server) accept(ln net.Listener) {
	defer s.wg.Done()
	var innerWg sync.WaitGroup
	for {
		if s.expectedConns == 0 {
			break
		}
		c, err := ln.Accept()
		if err != nil {
			c.Close()
			panic(err)
		}
		innerWg.Add(1)
		go s.handle(c, &innerWg)
		s.expectedConns--
	}
	innerWg.Wait()
}

func (s *server) handle(conn net.Conn, innerWg *sync.WaitGroup) {
	defer func() {
		conn.Close()
		innerWg.Done()
	}()
	buf := make([]byte, 4096)
	for {
		n, err := conn.Read(buf)
		if err != nil {
			if err != io.EOF {
				panic(err)
			} else {
				break
			}
		}
		if n < 1 {
			continue
		}
		buf = buf[:n]
		s.writeChan <- buf
		break
	}
}
