package sonic

import (
	"fmt"
	"net"
	"strings"
	"syscall"
	"testing"
)

var msg = []byte("hello, sonic!")

func TestRead(t *testing.T) {
	ioc := MustIO()
	defer ioc.Close()

	ln, err := net.Listen("tcp", "localhost:8080")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	go func() {
		client, err := ln.Accept()
		if err != nil {
			panic(err)
		}
		defer client.Close()

		client.Write(msg)
	}()

	client, err := net.Dial("tcp", "localhost:8080")
	if err != nil {
		t.Fatal(err)
	}

	sc := client.(syscall.Conn)
	NewAsyncAdapter(ioc, sc, client, func(err error, adapter *AsyncAdapter) {
		if err != nil {
			t.Fatal(err)
		}

		buf := make([]byte, 128)
		n, err := adapter.Read(buf)
		if err != nil {
			t.Fatal(err)
		}
		buf = buf[:n]

		smsg := string(msg)
		sbuf := string(buf)
		if !strings.EqualFold(smsg, sbuf) {
			t.Fatalf("short read expected=%s given=%s", smsg, sbuf)
		}
	})
}

func TestAsyncRead(t *testing.T) {
	ioc := MustIO()
	defer ioc.Close()

	ln, err := net.Listen("tcp", "localhost:8080")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	go func() {
		client, err := ln.Accept()
		if err != nil {
			panic(err)
		}
		defer client.Close()

		client.Write(msg)
	}()

	client, err := net.Dial("tcp", "localhost:8080")
	if err != nil {
		t.Fatal(err)
	}

	invoked := false

	sc := client.(syscall.Conn)
	NewAsyncAdapter(ioc, sc, client, func(err error, adapter *AsyncAdapter) {
		if err != nil {
			t.Fatal(err)
		}

		buf := make([]byte, 128)
		adapter.AsyncRead(buf, func(err error, n int) {
			invoked = true

			if err != nil {
				t.Fatal(err)
			}

			buf = buf[:n]

			smsg := string(msg)
			sbuf := string(buf)
			if !strings.EqualFold(smsg, sbuf) {
				t.Fatalf("short read expected=%s given=%s", smsg, sbuf)
			}
		})
	})

	ioc.RunOne()

	if !invoked {
		t.Fatalf("AsyncRead completion handler not invoked. Did you call ioc.Run*/ioc.Poll*?")
	}
}

func TestAsyncReadAll(t *testing.T) {
	ioc := MustIO()
	defer ioc.Close()

	ln, err := net.Listen("tcp", "localhost:8080")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	go func() {
		client, err := ln.Accept()
		if err != nil {
			panic(err)
		}
		defer client.Close()

		client.Write(msg)
	}()

	client, err := net.Dial("tcp", "localhost:8080")
	if err != nil {
		t.Fatal(err)
	}

	invoked := false

	sc := client.(syscall.Conn)
	NewAsyncAdapter(ioc, sc, client, func(err error, adapter *AsyncAdapter) {
		if err != nil {
			t.Fatal(err)
		}

		buf := make([]byte, len(msg))
		adapter.AsyncReadAll(buf, func(err error, n int) {
			invoked = true

			if err != nil {
				t.Fatal(err)
			}

			smsg := string(msg)
			sbuf := string(buf)
			if !strings.EqualFold(smsg, sbuf) {
				t.Fatalf("short read expected=%s given=%s", smsg, sbuf)
			}
		})
	})

	ioc.RunOne()

	if !invoked {
		t.Fatalf("AsyncReadAll completion handler not invoked. Did you call ioc.Run*/ioc.Poll*?")
	}
}

func TestWrite(t *testing.T) {
	ioc := MustIO()
	defer ioc.Close()

	ln, err := net.Listen("tcp", "localhost:8080")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	go func() {
		client, err := ln.Accept()
		if err != nil {
			panic(err)
		}
		defer client.Close()

		buf := make([]byte, 128)
		n, err := client.Read(buf)
		if err != nil {
			panic(err)
		}
		buf = buf[:n]

		smsg := string(msg)
		sbuf := string(buf)
		if !strings.EqualFold(smsg, sbuf) {
			panic(fmt.Errorf("short read expected=%s given=%s", smsg, sbuf))
		}
	}()

	client, err := net.Dial("tcp", "localhost:8080")
	if err != nil {
		t.Fatal(err)
	}

	sc := client.(syscall.Conn)
	NewAsyncAdapter(ioc, sc, client, func(err error, adapter *AsyncAdapter) {
		if err != nil {
			t.Fatal(err)
		}

		n, err := adapter.Write(msg)
		if err != nil {
			t.Fatal(err)
		}
		if n != len(msg) {
			t.Fatalf("short write expected=%d given=%d", len(msg), n)
		}
	})
}

func TestAsyncWrite(t *testing.T) {
	ioc := MustIO()
	defer ioc.Close()

	ln, err := net.Listen("tcp", "localhost:8080")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	go func() {
		client, err := ln.Accept()
		if err != nil {
			panic(err)
		}
		defer client.Close()

		buf := make([]byte, 128)
		n, err := client.Read(buf)
		if err != nil {
			panic(err)
		}
		buf = buf[:n]

		smsg := string(msg)
		sbuf := string(buf)
		if !strings.EqualFold(smsg, sbuf) {
			panic(fmt.Errorf("short read expected=%s given=%s", smsg, sbuf))
		}
	}()

	client, err := net.Dial("tcp", "localhost:8080")
	if err != nil {
		t.Fatal(err)
	}

	invoked := false

	sc := client.(syscall.Conn)
	NewAsyncAdapter(ioc, sc, client, func(err error, adapter *AsyncAdapter) {
		if err != nil {
			t.Fatal(err)
		}

		adapter.AsyncWrite(msg, func(err error, n int) {
			invoked = true

			if err != nil {
				t.Fatal(err)
			}

			if n != len(msg) {
				t.Fatalf("short write expected=%d given=%d", len(msg), n)
			}
		})
	})

	ioc.RunOne()

	if !invoked {
		t.Fatalf("AsyncWrite completion handler not invoked. Did you call ioc.Run*/ioc.Poll*?")
	}
}

func TestAsyncWriteAll(t *testing.T) {
	ioc := MustIO()
	defer ioc.Close()

	ln, err := net.Listen("tcp", "localhost:8080")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	go func() {
		client, err := ln.Accept()
		if err != nil {
			panic(err)
		}
		defer client.Close()

		buf := make([]byte, 128)
		n, err := client.Read(buf)
		if err != nil {
			panic(err)
		}
		buf = buf[:n]

		smsg := string(msg)
		sbuf := string(buf)
		if !strings.EqualFold(smsg, sbuf) {
			panic(fmt.Errorf("short read expected=%s given=%s", smsg, sbuf))
		}
	}()

	client, err := net.Dial("tcp", "localhost:8080")
	if err != nil {
		t.Fatal(err)
	}

	invoked := false

	sc := client.(syscall.Conn)
	NewAsyncAdapter(ioc, sc, client, func(err error, adapter *AsyncAdapter) {
		if err != nil {
			t.Fatal(err)
		}

		adapter.AsyncWriteAll(msg, func(err error, n int) {
			invoked = true

			if err != nil {
				t.Fatal(err)
			}

			if n != len(msg) {
				t.Fatalf("short write expected=%d given=%d", len(msg), n)
			}
		})
	})

	ioc.RunOne()

	if !invoked {
		t.Fatalf("AsyncWriteAll completion handler not invoked. Did you call ioc.Run*/ioc.Poll*?")
	}
}
