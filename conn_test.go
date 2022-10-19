package sonic

import (
	"errors"
	"fmt"
	"github.com/talostrading/sonic/sonicerrors"
	"github.com/talostrading/sonic/sonicopts"
	"golang.org/x/sys/unix"
	"io"
	"net"
	"syscall"
	"testing"
)

func TestConn_SyncErrWouldBlock(t *testing.T) {
	flag := make(chan struct{})

	go func() {
		ln, err := net.Listen("tcp", "localhost:8080")
		if err != nil {
			panic(err)
		}
		defer func() {
			ln.Close()
			flag <- struct{}{}
		}()
		flag <- struct{}{}

		_, err = ln.Accept()
		if err != nil {
			panic(err)
		}
	}()
	<-flag

	ioc := MustIO()
	defer ioc.Close()

	conn, err := Dial(ioc, "tcp", "localhost:8080", sonicopts.Nonblocking(true))
	if err != nil {
		t.Fatal(err)
	}

	b := make([]byte, 128)

	_, err = conn.Read(b)
	if !errors.Is(err, sonicerrors.ErrWouldBlock) {
		t.Fatalf("expected ErrWouldBlock but got %v", err)
	}

	<-flag
}

func TestConn_AsyncErrWouldBlock(t *testing.T) {
	flag := make(chan struct{})

	go func() {
		ln, err := net.Listen("tcp", "localhost:8080")
		if err != nil {
			panic(err)
		}
		defer func() {
			ln.Close()
			flag <- struct{}{}
		}()
		flag <- struct{}{}

		_, err = ln.Accept()
		if err != nil {
			panic(err)
		}
	}()
	<-flag

	ioc := MustIO()
	defer ioc.Close()

	conn, err := Dial(ioc, "tcp", "localhost:8080", sonicopts.Nonblocking(true))
	if err != nil {
		t.Fatal(err)
	}

	b := make([]byte, 128)

	conn.AsyncRead(b, func(_ error, _ int) {})

	ioc.PollOne()

	if ioc.Pending() != 1 {
		t.Fatal("should have one pending read")
	}

	<-flag
}

func TestConn_IsBlocking(t *testing.T) {
	testConnBlockingFlag(t, false)
}

func TestConn_IsNonblocking(t *testing.T) {
	testConnBlockingFlag(t, true)
}

func TestConn_NonblockingAsyncReadError(t *testing.T) {
	testConnAsyncReadError(t, true)
}

func TestConn_BlockingAsyncReadError(t *testing.T) {
	testConnAsyncReadError(t, false)
}

func TestConn_NonblockingAsyncWriteError(t *testing.T) {
	testConnAsyncWriteError(t, true)
}

func TestConn_BlockingAsyncWriteError(t *testing.T) {
	testConnAsyncWriteError(t, false)
}

func TestConn_NonblockingAsyncReadWrite(t *testing.T) {
	testAsyncReadWrite(t, true)
}

func TestConn_BlockingAsyncReadWrite(t *testing.T) {
	testAsyncReadWrite(t, false)
}

func testConnBlockingFlag(t *testing.T, nonblocking bool) {
	flag := make(chan struct{})

	go func() {
		ln, err := net.Listen("tcp", "localhost:8080")
		if err != nil {
			panic(err)
		}
		defer func() {
			ln.Close()
			flag <- struct{}{}
		}()
		flag <- struct{}{}

		conn, err := ln.Accept()
		if err != nil {
			panic(err)
		}
		defer conn.Close()
	}()
	<-flag

	ioc := MustIO()
	defer ioc.Close()

	conn, err := Dial(ioc, "tcp", "localhost:8080", sonicopts.Nonblocking(nonblocking))
	if err != nil {
		t.Fatal(err)
	}

	ret, err := unix.FcntlInt(uintptr(conn.RawFd()), unix.F_GETFL, 0)
	if err != nil {
		t.Fatal(err)
	}

	if nonblocking && ret&syscall.O_NONBLOCK == 0 {
		t.Fatal("connection should be nonblocking")
	} else if !nonblocking && ret&syscall.O_NONBLOCK != 0 {
		t.Fatal("connection should be blocking")
	}

	<-flag
}

func testConnAsyncReadError(t *testing.T, nonblocking bool) {
	flag := make(chan struct{})

	go func() {
		ln, err := net.Listen("tcp", "localhost:8080")
		if err != nil {
			panic(err)
		}
		defer func() {
			ln.Close()
			flag <- struct{}{}
		}()
		flag <- struct{}{}

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
	<-flag

	ioc := MustIO()
	defer ioc.Close()

	conn, err := Dial(ioc, "tcp", "localhost:8080", sonicopts.Nonblocking(nonblocking))
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

	<-flag

	if !done {
		t.Fatal("test did not run to completion")
	}
}

func testConnAsyncWriteError(t *testing.T, nonblocking bool) {
	flag := make(chan struct{})

	go func() {
		ln, err := net.Listen("tcp", "localhost:8080")
		if err != nil {
			panic(err)
		}
		defer func() {
			ln.Close()
			flag <- struct{}{}
		}()
		flag <- struct{}{}

		conn, err := ln.Accept()
		if err != nil {
			panic(err)
		}

		b := make([]byte, 128)
		_, err = conn.Read(b)
		if err != nil {
			panic(err)
		}

		err = conn.Close()
		if err != nil {
			panic(err)
		}
	}()
	<-flag

	ioc := MustIO()
	defer ioc.Close()

	conn, err := Dial(ioc, "tcp", "localhost:8080", sonicopts.Nonblocking(nonblocking))
	if err != nil {
		t.Fatal(err)
	}

	done := false
	var onAsyncWrite AsyncCallback
	onAsyncWrite = func(err error, n int) {
		if err != nil {
			if errors.Is(err, io.EOF) || errors.Is(err, syscall.EPIPE) || errors.Is(err, syscall.ECONNRESET) {
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

	<-flag

	if !done {
		t.Fatal("test did not run to completion")
	}
}

func testAsyncReadWrite(t *testing.T, nonblocking bool) {
	flag := make(chan struct{})

	go func() {
		ln, err := net.Listen("tcp", "localhost:8080")
		if err != nil {
			panic(err)
		}
		defer func() {
			ln.Close()
			flag <- struct{}{}
		}()
		flag <- struct{}{}

		conn, err := ln.Accept()
		if err != nil {
			panic(err)
		}
		defer conn.Close()

		b := make([]byte, 128)
		for {
			_, err = conn.Write([]byte("hello"))
			if err != nil {
				break
			}

			b = b[:cap(b)]
			n, err := conn.Read(b)
			if err != nil {
				break
			}

			if string(b[:n]) != "hello" {
				panic(fmt.Errorf("did not read %v", string(b)))
			}
		}
	}()
	<-flag

	ioc := MustIO()
	defer ioc.Close()

	conn, err := Dial(ioc, "tcp", "localhost:8080", sonicopts.Nonblocking(nonblocking))
	if err != nil {
		panic(err)
	}

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
					if !errors.Is(err, io.EOF) || !errors.Is(err, syscall.EPIPE) || !errors.Is(err, syscall.ECONNRESET) {
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
	conn.Close()

	<-flag
}
