package sonicwebsocket

import (
	"errors"
	"fmt"
	"sync/atomic"
	"testing"

	"github.com/talostrading/sonic"
)

func TestClientAsyncHandshake(t *testing.T) {
	srv := &testServer{}

	var done uint32 = 0

	go func() {
		defer func() {
			atomic.StoreUint32(&done, 1)
			srv.Close()
		}()

		err := srv.Accept("localhost:8080")
		if err != nil {
			panic(err)
		}
	}()

	ioc := sonic.MustIO()
	defer ioc.Close()

	ws, err := NewWebsocketStream(ioc, nil, RoleClient)
	if err != nil {
		t.Fatal(err)
	}

	if expect := StateTerminated; ws.State() != expect {
		t.Fatalf("wrong websocket state expected=%s given=%s", expect, ws.State())
	}

	ws.AsyncHandshake("ws://localhost:8080", func(err error) {
		if err != nil {
			t.Fatal(err)
		} else {

			if expect := StateActive; ws.State() != expect {
				t.Fatalf("wrong websocket state expected=%s given=%s", expect, ws.State())
			}
		}
	})

	for {
		ioc.RunOne()
		if atomic.LoadUint32(&done) == 1 {
			break
		}
	}
}

func TestClientAsyncReadSingleFrame(t *testing.T) {
	expected := "hello"

	var done uint32 = 0

	srv := &testServer{}
	go func() {
		defer func() {
			atomic.StoreUint32(&done, 1)
			srv.Close()
		}()

		err := srv.Accept("localhost:8081")
		if err != nil {
			panic(err)
		}

		fr := AcquireFrame()
		defer ReleaseFrame(fr)

		fr.SetText()
		fr.SetFin()
		fr.SetPayload([]byte(expected))

		_, err = srv.Write(fr)
		if err != nil {
			panic(err)
		}
	}()

	ioc := sonic.MustIO()
	defer ioc.Close()

	ws, err := NewWebsocketStream(ioc, nil, RoleClient)
	if err != nil {
		t.Fatal(err)
	}

	ws.AsyncHandshake("ws://localhost:8081", func(err error) {
		if err != nil {
			t.Fatal(err)
		} else {
			b := make([]byte, 4096)
			ws.AsyncReadSome(b, func(err error, n int, mt MessageType) {
				if err != nil {
					t.Fatal(err)
				} else {
					if expect := StateActive; ws.State() != expect {
						t.Fatalf("wrong websocket state expected=%s given=%s", expect, ws.State())
					}

					b = b[:n]
					if given := string(b); given != expected {
						t.Fatalf("invalid read expected=%s given=%s", expected, given)
					}

					if expect := TypeText; mt != expect {
						t.Fatalf("wrong message type expected=%s given=%s", expect, mt)
					}
				}
			})
		}
	})

	for {
		ioc.RunOne()
		if atomic.LoadUint32(&done) == 1 {
			break
		}
	}
}

func TestClientAsyncReadMaskedFrame(t *testing.T) {
	expected := "hello"

	var done uint32 = 0

	srv := &testServer{}
	go func() {
		defer func() {
			atomic.StoreUint32(&done, 1)
			srv.Close()
		}()

		err := srv.Accept("localhost:8082")
		if err != nil {
			panic(err)
		}

		fr := AcquireFrame()
		defer ReleaseFrame(fr)

		fr.SetText()
		fr.SetFin()
		fr.SetPayload([]byte(expected))
		fr.Mask(nil)

		_, err = srv.Write(fr)
		if err != nil {
			panic(err)
		}
	}()

	ioc := sonic.MustIO()
	defer ioc.Close()

	ws, err := NewWebsocketStream(ioc, nil, RoleClient)
	if err != nil {
		t.Fatal(err)
	}

	ws.AsyncHandshake("ws://localhost:8082", func(err error) {
		if err != nil {
			t.Fatal(err)
		} else {
			b := make([]byte, 10)
			ws.AsyncReadSome(b, func(err error, n int, mt MessageType) {
				if expect := ErrMaskedFrameFromServer; !errors.Is(err, expect) {
					t.Fatalf("wrong error expected=%s given=%s", expect, err)
				}

				if expect := TypeText; mt != expect {
					t.Fatalf("wrong message type expected=%s given=%s", expect, mt)
				}
			})
		}
	})

	for {
		ioc.RunOne()
		if atomic.LoadUint32(&done) == 1 {
			break
		}
	}
}

func TestClientAsyncClose(t *testing.T) {
	expected := "hello"

	var done uint32 = 0

	srv := &testServer{}
	go func() {
		defer func() {
			atomic.StoreUint32(&done, 1)
			srv.Close()
		}()

		err := srv.Accept("localhost:8083")
		if err != nil {
			panic(err)
		}

		fr := AcquireFrame()
		defer ReleaseFrame(fr)

		fr.SetText()
		fr.SetFin()
		fr.SetPayload([]byte(expected))

		_, err = srv.Write(fr)
		if err != nil {
			panic(err)
		}

		fr.Reset()
		fr.SetFin()
		fr.SetClose()

		_, err = srv.Write(fr)
		if err != nil {
			panic(err)
		}

		fr.Reset()
		_, err = srv.Read(fr)
		if err != nil {
			panic(err)
		}
	}()

	ioc := sonic.MustIO()
	defer ioc.Close()

	AsyncNewTestClient(ioc, "ws://localhost:8083", func(err error, cl *testClient) {
		if err != nil {
			t.Fatal(err)
		} else {
			b := make([]byte, 128)
			cl.RunReadLoop(b, func(err error, n int, mt MessageType) {
				b = b[:n]
				fmt.Println("received", b, err, n, mt)

				if mt == TypeClose {
					if expect := StateClosedByPeer; cl.ws.State() != expect {
						t.Fatalf("wrong state expected=%s given=%s", expect, cl.ws.State())
					}
				}
			})
		}
	})

	for {
		ioc.RunOne()
		if atomic.LoadUint32(&done) == 1 {
			break
		}
	}
}
