package sonic

import (
	"errors"
	"fmt"
	"io"
	"net"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/talostrading/sonic/sonicopts"
)

func TestConnUDPAsyncWrite(t *testing.T) {
	var nread uint32 = 0
	marker := make(chan struct{}, 1)
	go func() {
		udpAddr, err := net.ResolveUDPAddr("udp", "localhost:8084")
		if err != nil {
			panic(err)
		}
		udp, err := net.ListenUDP("udp", udpAddr)
		if err != nil {
			panic(err)
		}
		defer udp.Close()

		marker <- struct{}{}
		<-marker

		b := make([]byte, 128)
		for i := 0; i < 1000; i++ {
			n, err := udp.Read(b)
			if err == nil {
				b = b[:n]
				if string(b) != "hello" {
					panic("invalid message")
				}
				atomic.AddUint32(&nread, 1)
			}
		}

		marker <- struct{}{} // done reading
	}()
	<-marker

	ioc := MustIO()
	defer ioc.Close()

	conn, err := Dial(ioc, "udp", "localhost:8084")
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	var onWrite AsyncCallback
	onWrite = func(err error, _ int) {
		if err != nil {
			if err != io.EOF {
				t.Fatal(err)
			}
		} else {
			select {
			case <-marker:
			default:
				conn.AsyncWriteAll([]byte("hello"), onWrite)
			}
		}
	}

	marker <- struct{}{} // server can start
	conn.AsyncWriteAll([]byte("hello"), onWrite)

	ioc.RunPending()
	if atomic.LoadUint32(&nread) == 0 {
		t.Fatal("did not read anything")
	}
}

func TestConnUDPAsyncRead(t *testing.T) {
	marker := make(chan struct{}, 1)
	go func() {
		udpAddr, err := net.ResolveUDPAddr("udp", "localhost:8085")
		if err != nil {
			panic(err)
		}
		udp, err := net.DialUDP("udp", nil, udpAddr)
		if err != nil {
			panic(err)
		}
		defer udp.Close()

		<-marker

		for i := 0; i < 100; i++ {
			udp.Write([]byte("hello"))
			time.Sleep(time.Millisecond)
		}

		marker <- struct{}{}
	}()

	ioc := MustIO()
	defer ioc.Close()

	conn, err := ListenPacket(ioc, "udp", "localhost:8085", sonicopts.ReuseAddr(true))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	nread := 0
	b := make([]byte, 128)
	var onRead AsyncReadCallbackPacket
	onRead = func(err error, n int, fromAddr net.Addr) {
		if err != nil {
			if err != io.EOF {
				t.Fatal(err)
			}
		} else {
			b = b[:n]
			if string(b) != "hello" {
				t.Fatal("wrong message")
			}

			nread++

			select {
			case <-marker:
			default:
				b = b[:cap(b)]
				conn.AsyncReadFrom(b, onRead)
			}
		}
	}

	conn.AsyncReadFrom(b, onRead)
	marker <- struct{}{}

	now := time.Now()
	for nread < 5 || time.Now().Sub(now) < time.Second {
		ioc.PollOne()
	}
	if nread == 0 {
		t.Fatal("did not read anything")
	}
}

func TestConnAsyncTCPEchoClient(t *testing.T) {
	marker := make(chan struct{}, 1)

	go func() {
		ln, err := net.Listen("tcp", "localhost:8086")
		if err != nil {
			panic(err)
		}
		defer ln.Close()

		marker <- struct{}{}

		conn, err := ln.Accept()
		if err != nil {
			panic(err)
		}
		defer conn.Close()

		b := make([]byte, 128)
	outer:
		for {
			select {
			case <-marker:
				break outer
			default:
			}

			conn.Write([]byte("hello"))

			b = b[:cap(b)]
			n, err := conn.Read(b)
			if err != nil {
				break outer
			}

			if string(b[:n]) != "hello" {
				panic(fmt.Errorf("did not read %v", string(b)))
			}

			time.Sleep(time.Millisecond)
		}
	}()
	<-marker

	ioc := MustIO()
	defer ioc.Close()

	conn, err := Dial(ioc, "tcp", "localhost:8086")
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	b := make([]byte, 5)
	var onAsyncRead AsyncCallback
	onAsyncRead = func(err error, n int) {
		if err != nil {
			if err != io.EOF {
				t.Fatal(err)
			}
		} else {
			b = b[:n]
			if string(b) != "hello" {
				t.Fatalf("did not read %v", string(b))
			}

			conn.AsyncWriteAll(b, func(err error, n int) {
				if err != nil {
					if !errors.Is(err, io.EOF) || !errors.Is(err, syscall.EPIPE) {
						t.Fatal(err)
					}
				} else {
					b = b[:5]
					conn.AsyncReadAll(b, onAsyncRead)
				}
			})
		}
	}

	conn.AsyncReadAll(b, onAsyncRead)

	for i := 0; i < 10; i++ {
		ioc.RunOne()
	}

	marker <- struct{}{}
}

func TestConnReadHandlesError(t *testing.T) {
	marker := make(chan struct{}, 1)
	go func() {
		ln, err := net.Listen("tcp", "localhost:8087")
		if err != nil {
			panic(err)
		}

		marker <- struct{}{}

		conn, err := ln.Accept()
		if err != nil {
			panic(err)
		}
		defer conn.Close()

		_, err = conn.Write([]byte("hello"))
		if err != nil {
			panic(err)
		}

	}()
	<-marker

	ioc := MustIO()
	defer ioc.Close()

	conn, err := Dial(ioc, "tcp", "localhost:8087")
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	done := false
	b := make([]byte, 128)
	var onAsyncRead AsyncCallback
	onAsyncRead = func(err error, n int) {
		if err != nil {
			if err != io.EOF {
				t.Fatal(err)
			} else {
				done = true
			}
		} else {
			b = b[:cap(b)]
			conn.AsyncReadAll(b, onAsyncRead)
		}
	}
	conn.AsyncReadAll(b, onAsyncRead)

	ioc.RunPending()

	if !done {
		t.Fatal("test did not run to completion")
	}
}

func TestConnWriteHandlesError(t *testing.T) {
	marker := make(chan struct{}, 1)

	go func() {
		ln, err := net.Listen("tcp", "localhost:8088")
		if err != nil {
			panic(err)
		}
		defer ln.Close()

		marker <- struct{}{}

		conn, err := ln.Accept()
		if err != nil {
			panic(err)
		}
		defer conn.Close()

		b := make([]byte, 128)
		_, err = conn.Read(b)
		if err != nil {
			panic(err)
		}
	}()
	<-marker

	ioc := MustIO()
	defer ioc.Close()

	conn, err := Dial(ioc, "tcp", "localhost:8088")
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	done := false
	var onAsyncWrite AsyncCallback
	onAsyncWrite = func(err error, n int) {
		if err != nil {
			if errors.Is(err, syscall.EPIPE) || errors.Is(err, syscall.ECONNRESET) {
				done = true
			} else {
				t.Fatal(err)
			}
		} else {
			conn.AsyncWriteAll([]byte("hello"), onAsyncWrite)
		}
	}
	conn.AsyncWriteAll([]byte("hello"), onAsyncWrite)

	ioc.RunPending()

	if !done {
		t.Fatal("test did not run to completion")
	}
}

func TestDispatchLimit(t *testing.T) {
	// Since the `ioc.Dispatched` counter is shared amongst, we test if it updated correctly when two connections build
	// up each other's stack-frames. As in, when connection 1 has an immediate async read, it invokes connection 2's
	// async read which is immediate and when done that invokes connection 1's async read which is immediate and so
	// on. We ensure we reach the `MaxCallbackDispatch` limit with this sequence of reads. We then assert that
	// `ioc.Dispatched` ends up 0 at the end of all operations.

	var (
		writtenBytes = MaxCallbackDispatch * 2
		readBytes    = 0
	)

	assert := assert.New(t)

	ioc := MustIO()
	defer ioc.Close()

	assert.Equal(0, ioc.Dispatched)

	// setup up the server which will write to both reading connections
	ln, err := net.Listen("tcp", "localhost:0")
	assert.Nil(err)
	addr := ln.Addr().String()

	marker := make(chan struct{}, 10)
	go func() {
		marker <- struct{}{}

		for {
			conn, err := ln.Accept()
			assert.Nil(err)

			go func() {
				// These bytes will get cached by the TCP layer. We thus ensure that each connection has at least one
				// series of `MaxCallbackDispatch` immediate asynchronous reads.
				for i := 0; i < writtenBytes; i++ {
					n, err := conn.Write([]byte("1"))
					assert.Nil(err)
					assert.Equal(1, n)
				}

				<-marker
				conn.Close()
			}()
		}
	}()
	<-marker

	limitHit := 0 // counts how many times each connection has hit the `MaxCallbackDispatch` limit

	var b [1]byte // shared between the two reading connections

	rd1, err := Dial(ioc, "tcp", addr)
	assert.Nil(err)
	defer rd1.Close()

	rd2, err := Dial(ioc, "tcp", addr)
	assert.Nil(err)
	defer rd2.Close()

	var (
		onRead1, onRead2 AsyncCallback
	)
	onRead1 = func(err error, n int) {
		if err != nil && err != io.EOF {
			t.Fatal(err)
		}
		if err == io.EOF {
			return
		}

		readBytes += n

		assert.True(ioc.Dispatched <= MaxCallbackDispatch)
		if ioc.Dispatched == MaxCallbackDispatch {
			limitHit++
		}

		// sleeping ensures the writer is faster than the reader so we build up stack frames by having each async read
		// complete immediately
		time.Sleep(time.Millisecond)

		// invoke the other reading connection's async read to ensure `ioc.Dispatched` is correctly updated when shared
		// between asynchronous objects
		rd2.AsyncRead(b[:], onRead2)
	}

	onRead2 = func(err error, n int) {
		if err != nil && err != io.EOF {
			t.Fatal(err)
		}
		if err == io.EOF {
			return
		}

		readBytes += n

		assert.True(ioc.Dispatched <= MaxCallbackDispatch)
		if ioc.Dispatched == MaxCallbackDispatch {
			limitHit++
		}

		time.Sleep(time.Millisecond)
		rd1.AsyncRead(b[:], onRead1) // the other connection
	}

	rd1.AsyncRead(b[:], onRead1) // starting point

	for readBytes < writtenBytes*2 /* two connections */ {
		ioc.PollOne()
	}

	assert.True(limitHit > 0)
	assert.Equal(0, ioc.Dispatched)

	marker <- struct{}{} // to close the write end
}
