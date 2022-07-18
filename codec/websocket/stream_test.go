package websocket

import (
	"bytes"
	"errors"
	"testing"

	"github.com/talostrading/sonic"
)

func TestClientHandshake(t *testing.T) {
	srv := &MockServer{}

	go func() {
		defer func() {
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

	if expect := StateHandshake; ws.State() != expect {
		t.Fatalf("wrong state expected=%s given=%s", expect, ws.State())
	}

	ws.AsyncHandshake("ws://localhost:8080", func(err error) {
		if err != nil {
			if expect := StateTerminated; ws.State() != expect {
				t.Fatalf("failed handshake with wrong state expected=%s given=%s", expect, ws.State())
			} else {
				t.Fatal(err)
			}
		} else {
			if expect := StateActive; ws.State() != expect {
				t.Fatalf("wrong state expected=%s given=%s", expect, ws.State())
			}
		}
	})

	for {
		ioc.RunOne()
		if srv.IsClosed() {
			break
		}
	}
}

func TestClientReadUnfragmentedMessage(t *testing.T) {
	ioc := sonic.MustIO()
	defer ioc.Close()

	ws, err := NewWebsocketStream(ioc, nil, RoleClient)
	if err != nil {
		t.Fatal(err)
	}

	// Skip the handshake and the stream initialization.
	// We put the messages in the buffer such that the codec stream
	// does not need to do any reads.
	ws.state = StateActive
	ws.init(nil)

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
}

func TestClientAsyncReadUnfragmentedMessage(t *testing.T) {
	ioc := sonic.MustIO()
	defer ioc.Close()

	ws, err := NewWebsocketStream(ioc, nil, RoleClient)
	if err != nil {
		t.Fatal(err)
	}

	ws.state = StateActive
	ws.init(nil)

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
}

func TestClientReadFragmentedMessage(t *testing.T) {
	ioc := sonic.MustIO()
	defer ioc.Close()

	ws, err := NewWebsocketStream(ioc, nil, RoleClient)
	if err != nil {
		t.Fatal(err)
	}

	ws.state = StateActive
	ws.init(nil)

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
}

func TestClientAsyncReadFragmentedMessage(t *testing.T) {
	ioc := sonic.MustIO()
	defer ioc.Close()

	ws, err := NewWebsocketStream(ioc, nil, RoleClient)
	if err != nil {
		t.Fatal(err)
	}

	ws.state = StateActive
	ws.init(nil)

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
}

func TestClientReadCorruptControlFrame(t *testing.T) {
	ioc := sonic.MustIO()
	defer ioc.Close()

	ws, err := NewWebsocketStream(ioc, nil, RoleClient)
	if err != nil {
		t.Fatal(err)
	}

	ws.state = StateActive
	ws.init(nil)

	ws.src.Write([]byte{
		byte(OpcodeClose), 2, 0x01, 0x02, // fin=false, type=close, payload_len=2
	})

	b := make([]byte, 128)
	mt, n, err := ws.NextMessage(b)
	b = b[:n]
	if mt != TypeClose {
		t.Fatal("wrong message type")
	}

	if err == nil || !errors.Is(err, ErrInvalidControlFrame) {
		t.Fatal("should have reported corrupt frame")
	}

	if ws.Pending() != 0 {
		t.Fatal("should have no pending operations")
	}
}

func TestClientAsyncReadCorruptControlFrame(t *testing.T) {
	ioc := sonic.MustIO()
	defer ioc.Close()

	ws, err := NewWebsocketStream(ioc, nil, RoleClient)
	if err != nil {
		t.Fatal(err)
	}

	ws.state = StateActive
	ws.init(nil)

	ws.src.Write([]byte{
		byte(OpcodeClose), 2, 0x01, 0x02, // fin=false, type=close, payload_len=2
	})

	b := make([]byte, 128)
	ran := false
	ws.AsyncNextMessage(b, func(err error, n int, mt MessageType) {
		ran = true
		if mt != TypeClose {
			t.Fatal("wrong message type")
		}

		if err == nil || !errors.Is(err, ErrInvalidControlFrame) {
			t.Fatal("should have reported corrupt frame")
		}

		if ws.Pending() != 0 {
			t.Fatal("should have no pending operations")
		}
	})

	if !ran {
		t.Fatal("async read did not run")
	}
}

func TestClientReadPingFrame(t *testing.T) {
	ioc := sonic.MustIO()
	defer ioc.Close()

	ws, err := NewWebsocketStream(ioc, nil, RoleClient)
	if err != nil {
		t.Fatal(err)
	}

	ws.state = StateActive
	ws.init(nil)

	ws.src.Write([]byte{
		byte(OpcodePing) | 1<<7, 2, 0x01, 0x02, // fin=true, type=ping, payload_len=2
	})

	b := make([]byte, 128)
	mt, n, err := ws.NextMessage(b)
	b = b[:n]
	if err != nil {
		t.Fatal(err)
	}
	if mt != TypePing {
		t.Fatal("wrong message type", mt)
	}
	if ws.Pending() != 1 {
		t.Fatal("should have a pending pong")
	}
	if !bytes.Equal(b, []byte{0x01, 0x02}) {
		t.Fatal("invalid payload")
	}

	reply := ws.pending[0]
	if !(reply.IsPong() && reply.IsMasked()) {
		t.Fatal("invalid pong reply")
	}

	reply.Unmask()
	if !bytes.Equal(reply.Payload(), []byte{0x01, 0x02}) {
		t.Fatal("invalid pong reply")
	}
}

func TestClientAsyncReadPingFrame(t *testing.T) {
	ioc := sonic.MustIO()
	defer ioc.Close()

	ws, err := NewWebsocketStream(ioc, nil, RoleClient)
	if err != nil {
		t.Fatal(err)
	}

	ws.state = StateActive
	ws.init(nil)

	ws.src.Write([]byte{
		byte(OpcodePing) | 1<<7, 2, 0x01, 0x02, // fin=true, type=ping, payload_len=2
	})

	b := make([]byte, 128)
	ran := false
	ws.AsyncNextMessage(b, func(err error, n int, mt MessageType) {
		ran = true

		b = b[:n]
		if err != nil {
			t.Fatal(err)
		}
		if mt != TypePing {
			t.Fatal("wrong message type", mt)
		}
		if ws.Pending() != 1 {
			t.Fatal("should have a pending pong")
		}
		if !bytes.Equal(b, []byte{0x01, 0x02}) {
			t.Fatal("invalid payload")
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

	if !ran {
		t.Fatal("async read did not run")
	}
}

func TestClientReadPongFrame(t *testing.T) {
	ioc := sonic.MustIO()
	defer ioc.Close()

	ws, err := NewWebsocketStream(ioc, nil, RoleClient)
	if err != nil {
		t.Fatal(err)
	}

	ws.state = StateActive
	ws.init(nil)

	ws.src.Write([]byte{
		byte(OpcodePong) | 1<<7, 2, 0x01, 0x02, // fin=true, type=ping, payload_len=2
	})

	b := make([]byte, 128)
	mt, n, err := ws.NextMessage(b)
	b = b[:n]
	if err != nil {
		t.Fatal(err)
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
}

func TestClientAsyncReadPongFrame(t *testing.T) {
	ioc := sonic.MustIO()
	defer ioc.Close()

	ws, err := NewWebsocketStream(ioc, nil, RoleClient)
	if err != nil {
		t.Fatal(err)
	}

	ws.state = StateActive
	ws.init(nil)

	ws.src.Write([]byte{
		byte(OpcodePong) | 1<<7, 2, 0x01, 0x02, // fin=true, type=ping, payload_len=2
	})

	b := make([]byte, 128)
	ran := false
	ws.AsyncNextMessage(b, func(err error, n int, mt MessageType) {
		ran = true

		b = b[:n]
		if err != nil {
			t.Fatal(err)
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

	if !ran {
		t.Fatal("async read did not run")
	}
}

func TestClientReadCloseFrame(t *testing.T) {
	ioc := sonic.MustIO()
	defer ioc.Close()

	ws, err := NewWebsocketStream(ioc, nil, RoleClient)
	if err != nil {
		t.Fatal(err)
	}

	ws.state = StateActive
	ws.init(nil)

	payload := EncodeCloseFramePayload(CloseNormal, "bye")
	ws.src.Write([]byte{
		byte(OpcodeClose) | 1<<7, byte(len(payload)),
	})
	ws.src.Write(payload)

	b := make([]byte, 128)
	mt, n, err := ws.NextMessage(b)
	b = b[:n]
	if err != nil {
		t.Fatal(err)
	}
	if mt != TypeClose {
		t.Fatal("wrong message type", mt)
	}
	if ws.Pending() != 1 {
		t.Fatal("should one pending operation")
	}
	if !bytes.Equal(b, payload) {
		t.Fatal("invalid payload")
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

	if ws.state != StateClosedByPeer {
		t.Fatal("invalid stream state")
	}
}

func TestClientAsyncReadCloseFrame(t *testing.T) {
	ioc := sonic.MustIO()
	defer ioc.Close()

	ws, err := NewWebsocketStream(ioc, nil, RoleClient)
	if err != nil {
		t.Fatal(err)
	}

	ws.state = StateActive
	ws.init(nil)

	payload := EncodeCloseFramePayload(CloseNormal, "bye")
	ws.src.Write([]byte{
		byte(OpcodeClose) | 1<<7, byte(len(payload)),
	})
	ws.src.Write(payload)

	b := make([]byte, 128)
	ran := false
	ws.AsyncNextMessage(b, func(err error, n int, mt MessageType) {
		ran = true

		b = b[:n]
		if err != nil {
			t.Fatal(err)
		}
		if mt != TypeClose {
			t.Fatal("wrong message type", mt)
		}
		if ws.Pending() != 1 {
			t.Fatal("should one pending operation")
		}
		if !bytes.Equal(b, payload) {
			t.Fatal("invalid payload")
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

		if ws.state != StateClosedByPeer {
			t.Fatal("invalid stream state")
		}
	})

	if !ran {
		t.Fatal("async read did not run")
	}
}
