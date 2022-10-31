package websocket

import (
	"bytes"
	"errors"
	"github.com/talostrading/sonic"
	"io"
	"net/url"
	"testing"
)

func assertState(t *testing.T, ws Stream, expected StreamState) {
	if ws.State() != expected {
		t.Fatalf("wrong state: given=%s expected=%s ", ws.State(), expected)
	}
}

func TestWebsocketStream_ClientSuccessfulHandshake(t *testing.T) {
	flag := make(chan struct{}, 1)
	defer func() {
		<-flag
	}()

	go func() {
		srv, err := NewMockServer("localhost:8080")
		if err != nil {
			panic(err)
		}
		defer func() {
			srv.Close()
			flag <- struct{}{}
		}()
		flag <- struct{}{}

		defer srv.Close()

		err = srv.Accept()
		if err != nil {
			panic(err)
		}
	}()
	<-flag

	ioc := sonic.MustIO()
	defer ioc.Close()

	ws, err := NewWebsocketStream(ioc)
	if err != nil {
		t.Fatal(err)
	}

	assertState(t, ws, StateHandshake)

	uri, err := url.Parse("ws://localhost:8080")
	if err != nil {
		t.Fatal(err)
	}

	conn, err := makeConn(ioc, uri.Host)
	if err != nil {
		t.Fatal(err)
	}

	invoked := false
	ws.AsyncHandshake(conn, uri, func(err error) {
		invoked = true
		if err != nil {
			assertState(t, ws, StateTerminated)
		} else {
			assertState(t, ws, StateActive)
		}
	})

	ioc.RunPending()

	if !invoked {
		t.Fatal("not invoked")
	}
}

func TestWebsocketStream_ClientReadUnFragmentedMessage(t *testing.T) {
	ioc := sonic.MustIO()
	defer ioc.Close()

	ws, err := NewWebsocketStream(ioc)
	if err != nil {
		t.Fatal(err)
	}

	// Skip the HandshakeClient and the stream initialization.
	// We put the messages in the buffer such that the codec stream
	// does not need to do any reads.
	ws.state = StateActive
	ws.role = RoleClient
	ws.prepare(NewMockConn())

	ws.src.Write([]byte{0x81, 2, 0x01, 0x02}) // fin=true type=text payload_len=2

	b := make([]byte, 128)
	mt, n, err := ws.NextMessage(b)
	if err != nil {
		t.Fatal(err)
	}
	if mt != TypeText {
		t.Fatal("wrong message type")
	}

	b = b[:n]
	if !bytes.Equal(b, []byte{0x01, 0x02}) {
		t.Fatal("wrong payload")
	}

	assertState(t, ws, StateActive)
}

func TestWebsocketStream_ClientAsyncReadUnFragmentedMessage(t *testing.T) {
	ioc := sonic.MustIO()
	defer ioc.Close()

	ws, err := NewWebsocketStream(ioc)
	if err != nil {
		t.Fatal(err)
	}

	ws.state = StateActive
	ws.role = RoleClient
	ws.prepare(NewMockConn())

	ws.src.Write([]byte{0x81, 2, 0x01, 0x02}) // fin=true type=text payload_len=2

	b := make([]byte, 128)
	ran := false
	ws.AsyncNextMessage(b, func(err error, n int, mt MessageType) {
		ran = true
		if err != nil {
			t.Fatal(err)
		} else {
			if mt != TypeText {
				t.Fatal("wrong message type")
			}

			b = b[:n]
			if !bytes.Equal(b, []byte{0x01, 0x02}) {
				t.Fatal("wrong payload")
			}
		}
	})

	if !ran {
		t.Fatal("async read did not run")
	}

	assertState(t, ws, StateActive)
}

func TestWebsocketStream_ClientReadFragmentedMessage(t *testing.T) {
	ioc := sonic.MustIO()
	defer ioc.Close()

	ws, err := NewWebsocketStream(ioc)
	if err != nil {
		t.Fatal(err)
	}

	ws.state = StateActive
	ws.role = RoleClient
	ws.prepare(NewMockConn())

	ws.src.Write([]byte{
		0x01, 2, 0x01, 0x02, // fin=false, type=text, payload_len=2
		0x80, 2, 0x03, 0x04, // fin=true, type=continuation payload_len=2
	})

	b := make([]byte, 128)
	mt, n, err := ws.NextMessage(b)
	if err != nil {
		t.Fatal(err)
	}
	if mt != TypeText {
		t.Fatal("wrong message type")
	}

	b = b[:n]
	if !bytes.Equal(b, []byte{0x01, 0x02, 0x03, 0x04}) {
		t.Fatal("wrong payload")
	}

	assertState(t, ws, StateActive)
}

func TestWebsocketStream_ClientAsyncReadFragmentedMessage(t *testing.T) {
	ioc := sonic.MustIO()
	defer ioc.Close()

	ws, err := NewWebsocketStream(ioc)
	if err != nil {
		t.Fatal(err)
	}

	ws.state = StateActive
	ws.role = RoleClient
	ws.prepare(NewMockConn())

	ws.src.Write([]byte{
		0x01, 2, 0x01, 0x02, // fin=false, type=text, payload_len=2
		0x80, 2, 0x03, 0x04, // fin=true, type=continuation payload_len=2
	})

	b := make([]byte, 128)
	ran := false
	ws.AsyncNextMessage(b, func(err error, n int, mt MessageType) {
		ran = true
		if err != nil {
			t.Fatal(err)
		} else {
			if mt != TypeText {
				t.Fatal("wrong message type")
			}

			b = b[:n]
			if !bytes.Equal(b, []byte{0x01, 0x02, 0x03, 0x04}) {
				t.Fatal("wrong payload")
			}
		}
	})

	if !ran {
		t.Fatal("async read did not run")
	}

	assertState(t, ws, StateActive)
}

func TestWebsocketStream_ClientReadCorruptControlFrame(t *testing.T) {
	ioc := sonic.MustIO()
	defer ioc.Close()

	ws, err := NewWebsocketStream(ioc)
	if err != nil {
		t.Fatal(err)
	}

	ws.state = StateActive
	ws.role = RoleClient
	ws.prepare(NewMockConn())

	ws.src.Write([]byte{
		byte(OpcodeClose), 2, 0x01, 0x02, // fin=false, type=close, payload_len=2
	})

	b := make([]byte, 128)
	mt, n, err := ws.NextMessage(b)
	b = b[:n]
	if mt != TypeNone {
		t.Fatal("wrong message type")
	}

	if err == nil || !errors.Is(err, ErrInvalidControlFrame) {
		t.Fatal("should have reported corrupt frame")
	}

	if ws.Pending() != 1 {
		t.Fatal("should have one pending operation")
	}

	assertState(t, ws, StateClosedByUs)

	closeFrame := ws.pending[0]
	closeFrame.Unmask()

	cc, _ := DecodeCloseFramePayload(ws.pending[0].payload)
	if cc != CloseProtocolError {
		t.Fatal("should have closed with protocol error")
	}
}

func TestWebsocketStream_ClientAsyncReadCorruptControlFrame(t *testing.T) {
	ioc := sonic.MustIO()
	defer ioc.Close()

	ws, err := NewWebsocketStream(ioc)
	if err != nil {
		t.Fatal(err)
	}

	ws.state = StateActive
	ws.role = RoleClient
	ws.prepare(NewMockConn())

	ws.src.Write([]byte{
		byte(OpcodeClose), 2, 0x01, 0x02, // fin=false, type=close, payload_len=2
	})

	b := make([]byte, 128)
	ran := false
	ws.AsyncNextMessage(b, func(err error, n int, mt MessageType) {
		ran = true
		if mt != TypeNone {
			t.Fatal("wrong message type")
		}

		if err == nil || !errors.Is(err, ErrInvalidControlFrame) {
			t.Fatal("should have reported corrupt frame")
		}

		// we should be notifying the server that it violated the protocol
		if ws.Pending() != 1 {
			t.Fatal("should have one pending operation")
		}

		assertState(t, ws, StateClosedByUs)

		closeFrame := ws.pending[0]
		closeFrame.Unmask()

		cc, _ := DecodeCloseFramePayload(ws.pending[0].payload)
		if cc != CloseProtocolError {
			t.Fatal("should have closed with protocol error")
		}
	})

	if !ran {
		t.Fatal("async read did not run")
	}
}

func TestWebsocketStream_ClientReadPingFrame(t *testing.T) {
	ioc := sonic.MustIO()
	defer ioc.Close()

	ws, err := NewWebsocketStream(ioc)
	if err != nil {
		t.Fatal(err)
	}

	ws.state = StateActive
	ws.role = RoleClient
	ws.prepare(NewMockConn())

	ws.src.Write([]byte{
		byte(OpcodePing) | 1<<7, 2, 0x01, 0x02, // fin=true, type=ping, payload_len=2
	})

	invoked := false
	ws.SetControlCallback(func(mt MessageType, b []byte) {
		invoked = true

		if !(mt == TypePing && bytes.Equal(b, []byte{1, 2})) {
			t.Fatal("invalid ping")
		}

		if ws.Pending() != 1 {
			t.Fatal("should have a pending pong")
		}

		reply := ws.pending[0]
		if !(reply.IsPong() && reply.IsMasked()) {
			t.Fatal("invalid pong reply")
		}

		reply.Unmask()
		if !bytes.Equal(reply.Payload(), []byte{0x01, 0x02}) {
			t.Fatal("invalid pong reply")
		}
	})

	b := make([]byte, 128)
	_, _, err = ws.NextMessage(b)
	if err != io.EOF {
		t.Fatalf("should have received EOF but got=%v", err)
	}

	if !invoked {
		t.Fatal("control callback not invoked")
	}

	assertState(t, ws, StateTerminated)
}

func TestWebsocketStream_ClientAsyncReadPingFrame(t *testing.T) {
	ioc := sonic.MustIO()
	defer ioc.Close()

	ws, err := NewWebsocketStream(ioc)
	if err != nil {
		t.Fatal(err)
	}

	ws.state = StateActive
	ws.role = RoleClient
	ws.prepare(NewMockConn())

	ws.src.Write([]byte{
		byte(OpcodePing) | 1<<7, 2, 0x01, 0x02, // fin=true, type=ping, payload_len=2
	})

	invoked := false
	ws.SetControlCallback(func(mt MessageType, b []byte) {
		invoked = true

		if !(mt == TypePing && bytes.Equal(b, []byte{1, 2})) {
			t.Fatal("invalid ping")
		}

		if ws.Pending() != 1 {
			t.Fatal("should have a pending pong")
		}

		reply := ws.pending[0]
		if !(reply.IsPong() && reply.IsMasked()) {
			t.Fatal("invalid pong reply")
		}

		reply.Unmask()
		if !bytes.Equal(reply.Payload(), []byte{0x01, 0x02}) {
			t.Fatal("invalid pong reply")
		}
	})

	b := make([]byte, 128)
	ran := false
	ws.AsyncNextMessage(b, func(err error, n int, mt MessageType) {
		ran = true
		if err != io.EOF {
			t.Fatal("should have received EOF")
		}

		assertState(t, ws, StateTerminated)
	})

	if !ran {
		t.Fatal("async read did not run")
	}

	if !invoked {
		t.Fatal("control callback not invoked")
	}
}

func TestWebsocketStream_ClientReadPongFrame(t *testing.T) {
	ioc := sonic.MustIO()
	defer ioc.Close()

	ws, err := NewWebsocketStream(ioc)
	if err != nil {
		t.Fatal(err)
	}

	ws.state = StateActive
	ws.role = RoleClient
	ws.prepare(NewMockConn())

	ws.src.Write([]byte{
		byte(OpcodePong) | 1<<7, 2, 0x01, 0x02, // fin=true, type=ping, payload_len=2
	})

	invoked := false
	ws.SetControlCallback(func(mt MessageType, b []byte) {
		invoked = true

		if !(mt == TypePong && bytes.Equal(b, []byte{1, 2})) {
			t.Fatal("invalid pong")
		}

		if mt != TypePong {
			t.Fatal("wrong message type", mt)
		}
		if ws.Pending() != 0 {
			t.Fatal("should have no pending operations")
		}
		if !bytes.Equal(b, []byte{0x01, 0x02}) {
			t.Fatal("invalid payload")
		}
	})

	b := make([]byte, 128)
	_, _, err = ws.NextMessage(b)
	if err != io.EOF {
		t.Fatal("should have received EOF")
	}

	assertState(t, ws, StateTerminated)

	if !invoked {
		t.Fatal("control callback not invoked")
	}
}

func TestWebsocketStream_ClientAsyncReadPongFrame(t *testing.T) {
	ioc := sonic.MustIO()
	defer ioc.Close()

	ws, err := NewWebsocketStream(ioc)
	if err != nil {
		t.Fatal(err)
	}

	ws.state = StateActive
	ws.role = RoleClient
	ws.prepare(NewMockConn())

	ws.src.Write([]byte{
		byte(OpcodePong) | 1<<7, 2, 0x01, 0x02, // fin=true, type=ping, payload_len=2
	})

	invoked := false
	ws.SetControlCallback(func(mt MessageType, b []byte) {
		invoked = true

		if !(mt == TypePong && bytes.Equal(b, []byte{1, 2})) {
			t.Fatal("invalid pong")
		}

		if ws.Pending() != 0 {
			t.Fatal("should have no pending operations")
		}
	})

	b := make([]byte, 128)
	ran := false
	ws.AsyncNextMessage(b, func(err error, n int, mt MessageType) {
		ran = true
		if err != io.EOF {
			t.Fatal(err)
		}

		assertState(t, ws, StateTerminated)
	})

	if !ran {
		t.Fatal("async read did not run")
	}

	if !invoked {
		t.Fatal("control callback not invoked")
	}
}

func TestWebsocketStream_ClientReadCloseFrame(t *testing.T) {
	ioc := sonic.MustIO()
	defer ioc.Close()

	ws, err := NewWebsocketStream(ioc)
	if err != nil {
		t.Fatal(err)
	}

	ws.state = StateActive
	ws.role = RoleClient
	ws.prepare(NewMockConn())

	payload := EncodeCloseFramePayload(CloseNormal, "bye")
	ws.src.Write([]byte{
		byte(OpcodeClose) | 1<<7, byte(len(payload)),
	})
	ws.src.Write(payload)

	invoked := false
	ws.SetControlCallback(func(mt MessageType, b []byte) {
		invoked = true

		if !(mt == TypeClose && bytes.Equal(b, payload)) {
			t.Fatal("invalid close reply", mt)
		}

		if ws.Pending() != 1 {
			t.Fatal("should have one pending operation")
		}

		reply := ws.pending[0]
		if !reply.IsMasked() {
			t.Fatal("reply should be masked")
		}
		reply.Unmask()

		cc, reason := DecodeCloseFramePayload(reply.payload)
		if !(cc == CloseNormal && reason == "bye") {
			t.Fatal("invalid close frame reply")
		}

		assertState(t, ws, StateClosedByPeer)
	})

	b := make([]byte, 128)
	_, _, err = ws.NextMessage(b)

	if len(ws.pending) > 0 {
		t.Fatal("should have flushed")
	}

	if !errors.Is(err, io.EOF) {
		t.Fatal("should have received EOF")
	}

	assertState(t, ws, StateClosedByPeer)

	if !invoked {
		t.Fatal("control callback not invoked")
	}
}

func TestWebsocketStream_ClientAsyncReadCloseFrame(t *testing.T) {
	ioc := sonic.MustIO()
	defer ioc.Close()

	ws, err := NewWebsocketStream(ioc)
	if err != nil {
		t.Fatal(err)
	}

	ws.state = StateActive
	ws.role = RoleClient
	ws.prepare(NewMockConn())

	payload := EncodeCloseFramePayload(CloseNormal, "bye")
	ws.src.Write([]byte{
		byte(OpcodeClose) | 1<<7, byte(len(payload)),
	})
	ws.src.Write(payload)

	invoked := false
	ws.SetControlCallback(func(mt MessageType, b []byte) {
		invoked = true

		if !(mt == TypeClose && bytes.Equal(b, payload)) {
			t.Fatal("invalid close reply", mt)
		}

		if ws.Pending() != 1 {
			t.Fatal("should have one pending operation")
		}

		reply := ws.pending[0]
		if !reply.IsMasked() {
			t.Fatal("reply should be masked")
		}
		reply.Unmask()

		cc, reason := DecodeCloseFramePayload(reply.payload)
		if !(cc == CloseNormal && reason == "bye") {
			t.Fatal("invalid close frame reply")
		}

		assertState(t, ws, StateClosedByPeer)
	})

	b := make([]byte, 128)
	ran := false
	ws.AsyncNextMessage(b, func(err error, n int, mt MessageType) {
		ran = true
		if !errors.Is(err, io.EOF) {
			t.Fatal("should have received EOF")
		}

		assertState(t, ws, StateTerminated)
	})

	if !ran {
		t.Fatal("async read did not run")
	}

	if !invoked {
		t.Fatal("control callback not invoked")
	}
}

func TestWebsocketStream_ClientWriteFrame(t *testing.T) {
	ioc := sonic.MustIO()
	defer ioc.Close()

	ws, err := NewWebsocketStream(ioc)
	if err != nil {
		t.Fatal(err)
	}

	ws.state = StateActive
	ws.role = RoleClient
	mock := NewMockConn()
	ws.prepare(mock)

	f := AcquireFrame()
	defer ReleaseFrame(f)
	f.SetFin()
	f.SetText()
	f.SetPayload([]byte{1, 2, 3, 4, 5})

	err = ws.WriteFrame(f)
	if err != nil {
		t.Fatal(err)
	} else {
		mock.b.Commit(mock.b.WriteLen())

		f := AcquireFrame()
		defer ReleaseFrame(f)

		_, err = f.ReadFrom(mock.b)
		if err != nil {
			t.Fatal(err)
		}

		if !(f.IsFin() && f.IsMasked() && f.IsText()) {
			t.Fatal("frame is corrupt, something went wrong with the encoder")
		}

		f.Unmask()

		if !bytes.Equal(f.Payload(), []byte{1, 2, 3, 4, 5}) {
			t.Fatal("frame payload is corrupt, something went wrong with the encoder")
		}

		assertState(t, ws, StateActive)
	}
}

func TestWebsocketStream_ClientAsyncWriteFrame(t *testing.T) {
	ioc := sonic.MustIO()
	defer ioc.Close()

	ws, err := NewWebsocketStream(ioc)
	if err != nil {
		t.Fatal(err)
	}

	ws.state = StateActive
	ws.role = RoleClient
	mock := NewMockConn()
	ws.prepare(mock)

	f := AcquireFrame()
	defer ReleaseFrame(f)
	f.SetFin()
	f.SetText()
	f.SetPayload([]byte{1, 2, 3, 4, 5})

	ran := false

	ws.AsyncWriteFrame(f, func(err error) {
		ran = true

		if err != nil {
			t.Fatal(err)
		} else {
			mock.b.Commit(mock.b.WriteLen())

			f := AcquireFrame()
			defer ReleaseFrame(f)

			_, err = f.ReadFrom(mock.b)
			if err != nil {
				t.Fatal(err)
			}

			if !(f.IsFin() && f.IsMasked() && f.IsText()) {
				t.Fatal("frame is corrupt, something went wrong with the encoder")
			}

			f.Unmask()

			if !bytes.Equal(f.Payload(), []byte{1, 2, 3, 4, 5}) {
				t.Fatal("frame payload is corrupt, something went wrong with the encoder")
			}

			assertState(t, ws, StateActive)
		}
	})

	if !ran {
		t.Fatal("async read did not run")
	}
}

func TestWebsocketStream_ClientWrite(t *testing.T) {
	ioc := sonic.MustIO()
	defer ioc.Close()

	ws, err := NewWebsocketStream(ioc)
	if err != nil {
		t.Fatal(err)
	}

	ws.state = StateActive
	ws.role = RoleClient
	mock := NewMockConn()
	ws.prepare(mock)

	err = ws.Write([]byte{1, 2, 3, 4, 5}, TypeText)
	if err != nil {
		t.Fatal(err)
	} else {
		mock.b.Commit(mock.b.WriteLen())

		f := AcquireFrame()
		defer ReleaseFrame(f)

		_, err = f.ReadFrom(mock.b)
		if err != nil {
			t.Fatal(err)
		}

		if !(f.IsFin() && f.IsMasked() && f.IsText()) {
			t.Fatal("frame is corrupt, something went wrong with the encoder")
		}

		f.Unmask()

		if !bytes.Equal(f.Payload(), []byte{1, 2, 3, 4, 5}) {
			t.Fatal("frame payload is corrupt, something went wrong with the encoder")
		}

		assertState(t, ws, StateActive)
	}
}

func TestWebsocketStream_ClientAsyncWrite(t *testing.T) {
	ioc := sonic.MustIO()
	defer ioc.Close()

	ws, err := NewWebsocketStream(ioc)
	if err != nil {
		t.Fatal(err)
	}

	ws.state = StateActive
	ws.role = RoleClient
	mock := NewMockConn()
	ws.prepare(mock)

	ws.AsyncWrite([]byte{1, 2, 3, 4, 5}, TypeText, func(err error) {
		if err != nil {
			t.Fatal(err)
		} else {
			mock.b.Commit(mock.b.WriteLen())

			f := AcquireFrame()
			defer ReleaseFrame(f)

			_, err = f.ReadFrom(mock.b)
			if err != nil {
				t.Fatal(err)
			}

			if !(f.IsFin() && f.IsMasked() && f.IsText()) {
				t.Fatal("frame is corrupt, something went wrong with the encoder")
			}

			f.Unmask()

			if !bytes.Equal(f.Payload(), []byte{1, 2, 3, 4, 5}) {
				t.Fatal("frame payload is corrupt, something went wrong with the encoder")
			}

			assertState(t, ws, StateActive)
		}
	})
}

func TestWebsocketStream_ClientClose(t *testing.T) {
	ioc := sonic.MustIO()
	defer ioc.Close()

	ws, err := NewWebsocketStream(ioc)
	if err != nil {
		t.Fatal(err)
	}

	ws.state = StateActive
	ws.role = RoleClient
	mock := NewMockConn()
	ws.prepare(mock)

	err = ws.Close(CloseNormal, "bye")
	if err != nil {
		t.Fatal(err)
	} else {
		mock.b.Commit(mock.b.WriteLen())

		f := AcquireFrame()
		defer ReleaseFrame(f)

		_, err = f.ReadFrom(mock.b)
		if err != nil {
			t.Fatal(err)
		}

		if !(f.IsFin() && f.IsMasked() && f.IsClose()) {
			t.Fatal("frame is corrupt, something went wrong with the encoder")
		}

		f.Unmask()

		if f.PayloadLen() != 5 {
			t.Fatal("wrong message in close frame")
		}

		cc, reason := DecodeCloseFramePayload(f.payload)
		if !(cc == CloseNormal && reason == "bye") {
			t.Fatal("wrong close frame payload")
		}
	}
}

func TestWebsocketStream_ClientAsyncClose(t *testing.T) {
	ioc := sonic.MustIO()
	defer ioc.Close()

	ws, err := NewWebsocketStream(ioc)
	if err != nil {
		t.Fatal(err)
	}

	ws.state = StateActive
	ws.role = RoleClient
	mock := NewMockConn()
	ws.prepare(mock)

	ran := false
	ws.AsyncClose(CloseNormal, "bye", func(err error) {
		ran = true

		if err != nil {
			t.Fatal(err)
		} else {
			mock.b.Commit(mock.b.WriteLen())

			f := AcquireFrame()
			defer ReleaseFrame(f)

			_, err = f.ReadFrom(mock.b)
			if err != nil {
				t.Fatal(err)
			}

			if !(f.IsFin() && f.IsMasked() && f.IsClose()) {
				t.Fatal("frame is corrupt, something went wrong with the encoder")
			}

			f.Unmask()

			if f.PayloadLen() != 5 {
				t.Fatal("wrong message in close frame")
			}

			cc, reason := DecodeCloseFramePayload(f.payload)
			if !(cc == CloseNormal && reason == "bye") {
				t.Fatal("wrong close frame payload")
			}
		}
	})

	if !ran {
		t.Fatal("async read did not run")
	}
}

func TestWebsocketStream_ClientCloseHandshakeWeStart(t *testing.T) {
	ioc := sonic.MustIO()
	defer ioc.Close()

	ws, err := NewWebsocketStream(ioc)
	if err != nil {
		t.Fatal(err)
	}

	ws.state = StateActive
	ws.role = RoleClient
	mock := NewMockConn()
	ws.prepare(mock)

	err = ws.Close(CloseNormal, "bye")
	if err != nil {
		t.Fatal(err)
	} else {
		mock.b.Commit(mock.b.WriteLen())

		serverReply := AcquireFrame()
		defer ReleaseFrame(serverReply)

		assertState(t, ws, StateClosedByUs)

		serverReply.SetFin()
		serverReply.SetClose()
		serverReply.SetPayload(EncodeCloseFramePayload(CloseNormal, "bye"))
		_, err = serverReply.WriteTo(ws.src)
		if err != nil {
			t.Fatal(err)
		}

		reply, err := ws.NextFrame()
		if err != nil {
			t.Fatal(err)
		}

		if !(reply.IsFin() && reply.IsClose()) {
			t.Fatal("wrong close reply")
		}

		cc, reason := DecodeCloseFramePayload(reply.payload)
		if !(cc == CloseNormal && reason == "bye") {
			t.Fatal("wrong close frame payload reply")
		}

		assertState(t, ws, StateCloseAcked)
	}
}

func TestWebsocketStream_ClientAsyncCloseHandshakeWeStart(t *testing.T) {
	ioc := sonic.MustIO()
	defer ioc.Close()

	ws, err := NewWebsocketStream(ioc)
	if err != nil {
		t.Fatal(err)
	}

	ws.state = StateActive
	ws.role = RoleClient
	mock := NewMockConn()
	ws.prepare(mock)

	ran := false
	ws.AsyncClose(CloseNormal, "bye", func(err error) {
		ran = true

		if err != nil {
			t.Fatal(err)
		} else {
			mock.b.Commit(mock.b.WriteLen())

			serverReply := AcquireFrame()
			defer ReleaseFrame(serverReply)

			serverReply.SetFin()
			serverReply.SetClose()
			serverReply.SetPayload(EncodeCloseFramePayload(CloseNormal, "bye"))
			_, err = serverReply.WriteTo(ws.src)
			if err != nil {
				t.Fatal(err)
			}

			reply, err := ws.NextFrame()
			if err != nil {
				t.Fatal(err)
			}

			if !(reply.IsFin() && reply.IsClose()) {
				t.Fatal("wrong close reply")
			}

			cc, reason := DecodeCloseFramePayload(reply.payload)
			if !(cc == CloseNormal && reason == "bye") {
				t.Fatal("wrong close frame payload reply")
			}

			assertState(t, ws, StateCloseAcked)
		}
	})

	if !ran {
		t.Fatal("async read did not run")
	}
}

func TestWebsocketStream_ClientCloseHandshakePeerStarts(t *testing.T) {
	ioc := sonic.MustIO()
	defer ioc.Close()

	ws, err := NewWebsocketStream(ioc)
	if err != nil {
		t.Fatal(err)
	}

	ws.state = StateActive
	ws.role = RoleClient
	mock := NewMockConn()
	ws.prepare(mock)

	serverClose := AcquireFrame()
	defer ReleaseFrame(serverClose)
	serverClose.SetFin()
	serverClose.SetClose()
	serverClose.SetPayload(EncodeCloseFramePayload(CloseNormal, "bye"))

	nn, err := serverClose.WriteTo(ws.src)
	if err != nil {
		t.Fatal(err)
	}

	ws.src.Commit(int(nn))

	recv, err := ws.NextFrame()
	if err != nil {
		t.Fatal(err)
	}

	if !recv.IsClose() {
		t.Fatal("should have received close")
	}

	assertState(t, ws, StateClosedByPeer)

	cc, reason := DecodeCloseFramePayload(recv.payload)
	if !(cc == CloseNormal && reason == "bye") {
		t.Fatal("peer close frame payload is corrupt")
	}

	if ws.Pending() != 1 {
		t.Fatal("should have a pending reply ready to be flushed")
	}
}

func TestWebsocketStream_ClientAsyncCloseHandshakePeerStarts(t *testing.T) {
	ioc := sonic.MustIO()
	defer ioc.Close()

	ws, err := NewWebsocketStream(ioc)
	if err != nil {
		t.Fatal(err)
	}

	ws.state = StateActive
	ws.role = RoleClient
	mock := NewMockConn()
	ws.prepare(mock)

	serverClose := AcquireFrame()
	defer ReleaseFrame(serverClose)
	serverClose.SetFin()
	serverClose.SetClose()
	serverClose.SetPayload(EncodeCloseFramePayload(CloseNormal, "bye"))

	nn, err := serverClose.WriteTo(ws.src)
	if err != nil {
		t.Fatal(err)
	}

	ws.src.Commit(int(nn))

	ws.AsyncNextFrame(func(err error, recv *Frame) {
		if err != nil {
			t.Fatal(err)
		}

		if !recv.IsClose() {
			t.Fatal("should have received close")
		}

		assertState(t, ws, StateClosedByPeer)

		cc, reason := DecodeCloseFramePayload(recv.payload)
		if !(cc == CloseNormal && reason == "bye") {
			t.Fatal("peer close frame payload is corrupt")
		}

		if ws.Pending() != 1 {
			t.Fatal("should have a pending reply ready to be flushed")
		}
	})
}
