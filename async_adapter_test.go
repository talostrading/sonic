package sonic

import (
	"net"
	"strings"
	"testing"
)

var msg = []byte("hello, sonic!")

func TestAsyncAdapter_Read(t *testing.T) {
	flag := make(chan struct{})
	defer func() { <-flag }()
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

		conn.Write(msg)
	}()
	<-flag

	ioc := MustIO()
	defer ioc.Close()

	conn, err := net.Dial("tcp", "localhost:8080")
	if err != nil {
		t.Fatal(err)
	}

	adapter, err := NewAsyncAdapter(ioc, conn)
	if err != nil {
		t.Fatal(err)
	}

	b := make([]byte, 128)

	n, err := adapter.Read(b)
	if err != nil {
		t.Fatal(err)
	}
	b = b[:n]

	if !strings.EqualFold(string(msg), string(b)) {
		t.Fatalf("short read expected=%s given=%s", string(msg), string(b))
	}
}

func TestAsyncAdapter_AsyncRead(t *testing.T) {
	flag := make(chan struct{})
	defer func() { <-flag }()
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

		conn.Write(msg)
	}()
	<-flag

	ioc := MustIO()
	defer ioc.Close()

	conn, err := net.Dial("tcp", "localhost:8080")
	if err != nil {
		t.Fatal(err)
	}

	adapter, err := NewAsyncAdapter(ioc, conn)
	if err != nil {
		t.Fatal(err)
	}

	b := make([]byte, 128)

	invoked := false
	adapter.AsyncRead(b, func(err error, n int) {
		invoked = true

		if err != nil {
			t.Fatal(err)
		}
		b = b[:n]

		if !strings.EqualFold(string(msg), string(b)) {
			t.Fatalf("short read expected=%s given=%s", string(msg), string(b))
		}
	})

	ioc.RunOne()

	if !invoked {
		t.Fatal("callback was not invoked")
	}
}

func TestAsyncAdapter_Write(t *testing.T) {
	flag := make(chan struct{})
	defer func() { <-flag }()
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
		n, err := conn.Read(b)
		if err != nil {
			panic(err)
		}
		b = b[:n]

		if !strings.EqualFold(string(msg), string(b)) {
			t.Fatalf("short read expected=%s given=%s", string(msg), string(b))
		}
	}()
	<-flag

	ioc := MustIO()
	defer ioc.Close()

	conn, err := net.Dial("tcp", "localhost:8080")
	if err != nil {
		t.Fatal(err)
	}

	adapter, err := NewAsyncAdapter(ioc, conn)
	if err != nil {
		t.Fatal(err)
	}

	_, err = adapter.Write(msg)
	if err != nil {
		t.Fatal(err)
	}
}

func TestAsyncAdapter_AsyncWrite(t *testing.T) {
	flag := make(chan struct{})
	defer func() { <-flag }()
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
		n, err := conn.Read(b)
		if err != nil {
			panic(err)
		}
		b = b[:n]

		if !strings.EqualFold(string(msg), string(b)) {
			t.Fatalf("short read expected=%s given=%s", string(msg), string(b))
		}
	}()
	<-flag

	ioc := MustIO()
	defer ioc.Close()

	conn, err := net.Dial("tcp", "localhost:8080")
	if err != nil {
		t.Fatal(err)
	}

	adapter, err := NewAsyncAdapter(ioc, conn)
	if err != nil {
		t.Fatal(err)
	}

	invoked := false
	adapter.AsyncWrite(msg, func(err error, n int) {
		invoked = true

		if err != nil {
			t.Fatal(err)
		}
	})

	ioc.RunOne()

	if !invoked {
		t.Fatal("callback was not invoked")
	}
}
