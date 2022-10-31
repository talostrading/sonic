package websocket

import (
	"crypto/sha1"
	"github.com/talostrading/sonic"
	"github.com/talostrading/sonic/codec/http"
	"net"
	"net/url"
	"testing"
	"time"
)

func TestHandshake_DoClient(t *testing.T) {
	extra := []byte("something")

	flag := make(chan struct{}, 1)
	defer func() {
		<-flag // wait for the listener to close
	}()

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

		b := sonic.NewByteBuffer()
		_, err = b.ReadFrom(conn)
		if err != nil {
			panic(err)
		}

		dec, err := http.NewRequestDecoder()
		if err != nil {
			panic(err)
		}

		req, err := dec.Decode(b)
		if err != nil {
			panic(err)
		}

		res, err := http.NewResponse()
		if err != nil {
			panic(err)
		}

		res.Proto = http.ProtoHttp11
		res.StatusCode = http.StatusSwitchingProtocols
		res.Status = http.StatusText(http.StatusSwitchingProtocols)
		res.Header.Add("Upgrade", "websocket")
		res.Header.Add("Connection", "Upgrade")
		res.Header.Add(
			"Sec-WebSocket-Accept",
			MakeServerResponseKey(sha1.New(), []byte(req.Header.Get("Sec-WebSocket-Key"))))

		enc, err := http.NewResponseEncoder()
		if err != nil {
			t.Fatal(err)
		}

		// Encode the response along with some random bytes. This checks whether the client can parse a buffer
		// containing the handshake response along with some bytes which might compose a frame/message.
		b.Reset()
		err = enc.Encode(res, b)
		if err != nil {
			panic(err)
		}
		b.Write(extra)

		b.CommitAll()

		// Write the response, but slowly. This checks if we can handle short reads on handshakes. When this happens,
		// The response decoder should return ErrNeedMore - the handshaker should simply retry to read more bytes in
		// order to compose the full handshake response.

		// Make sure to write the extra bytes along with the ones needed to complete the response.
		n := b.ReadLen() - 2*len(extra)
		i := 0
		for i < n {
			time.Sleep(time.Millisecond)

			_, err = conn.Write(b.Data()[i : i+1])
			if err != nil {
				panic(err)
			}
			i++
		}
		conn.Write(b.Data()[i:])
	}()
	<-flag

	ioc := sonic.MustIO()
	defer ioc.Close()

	handshake, err := NewHandshake(sonic.NewByteBuffer())
	if err != nil {
		t.Fatal(err)
	}

	uri, err := url.Parse("ws://localhost:8080")
	if err != nil {
		t.Fatal(err)
	}

	conn, err := net.Dial("tcp", uri.Host)
	if err != nil {
		t.Fatal(err)
	}
	adapter, err := sonic.AdaptNetConn(ioc, conn)
	if err != nil {
		t.Fatal(err)
	}

	err = handshake.Do(adapter, uri, RoleClient)
	if err != nil {
		t.Fatal(err)
	}

	handshake.b.CommitAll()
	if string(handshake.b.Data()) != string(extra) {
		t.Fatal("did not read extra bytes in handshake correctly")
	}
}

func TestHandshake_AsyncDoClient(t *testing.T) {
	extra := []byte("something")

	flag := make(chan struct{}, 1)
	defer func() {
		<-flag // wait for the listener to close
	}()

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

		b := sonic.NewByteBuffer()
		_, err = b.ReadFrom(conn)
		if err != nil {
			panic(err)
		}

		dec, err := http.NewRequestDecoder()
		if err != nil {
			panic(err)
		}

		req, err := dec.Decode(b)
		if err != nil {
			panic(err)
		}

		res, err := http.NewResponse()
		if err != nil {
			panic(err)
		}

		res.Proto = http.ProtoHttp11
		res.StatusCode = http.StatusSwitchingProtocols
		res.Status = http.StatusText(http.StatusSwitchingProtocols)
		res.Header.Add("Upgrade", "websocket")
		res.Header.Add("Connection", "Upgrade")
		res.Header.Add(
			"Sec-WebSocket-Accept",
			MakeServerResponseKey(sha1.New(), []byte(req.Header.Get("Sec-WebSocket-Key"))))

		enc, err := http.NewResponseEncoder()
		if err != nil {
			t.Fatal(err)
		}

		// Encode the response along with some random bytes. This checks whether the client can parse a buffer
		// containing the handshake response along with some bytes which might compose a frame/message.
		b.Reset()
		err = enc.Encode(res, b)
		if err != nil {
			panic(err)
		}
		b.Write(extra)

		b.CommitAll()

		// Write the response, but slowly. This checks if we can handle short reads on handshakes. When this happens,
		// The response decoder should return ErrNeedMore - the handshaker should simply retry to read more bytes in
		// order to compose the full handshake response.

		// Make sure to write the extra bytes along with the ones needed to complete the response.
		n := b.ReadLen() - 2*len(extra)
		i := 0
		for i < n {
			time.Sleep(time.Millisecond)

			_, err = conn.Write(b.Data()[i : i+1])
			if err != nil {
				panic(err)
			}
			i++
		}
		conn.Write(b.Data()[i:])
	}()
	<-flag

	ioc := sonic.MustIO()
	defer ioc.Close()

	handshake, err := NewHandshake(sonic.NewByteBuffer())
	if err != nil {
		t.Fatal(err)
	}

	uri, err := url.Parse("ws://localhost:8080")
	if err != nil {
		t.Fatal(err)
	}

	conn, err := makeConn(ioc, uri.Host)
	if err != nil {
		t.Fatal(err)
	}

	invoked := false
	handshake.AsyncDo(conn, uri, RoleClient, func(err error) {
		if err != nil {
			t.Fatal(err)
		}

		handshake.b.CommitAll()
		if string(handshake.b.Data()) != string(extra) {
			t.Fatal("did not read extra bytes in handshake correctly")
		}

		invoked = true
	})

	ioc.RunPending()

	if !invoked {
		t.Fatal("handshake not completed")
	}
}
