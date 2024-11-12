package websocket

import (
	"bytes"
	"crypto/tls"
	"errors"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/talostrading/sonic"
)

func assertState(t *testing.T, ws *Stream, expected StreamState) {
	if ws.State() != expected {
		t.Fatalf("wrong state: given=%s expected=%s ", ws.State(), expected)
	}
}

func TestClientServerSendsInvalidCloseCode(t *testing.T) {
	assert := assert.New(t)

	go func() {
		srv := &MockServer{}
		defer srv.Close()

		err := srv.Accept("localhost:8080")
		if err != nil {
			panic(err)
		}

		{
			frame := NewFrame()

			closeCode := CloseReserved1
			assert.False(ValidCloseCode(closeCode))

			frame.
				SetFIN().
				SetClose().
				SetPayload(EncodeCloseFramePayload(closeCode, "something"))

			frame.WriteTo(srv.conn)
		}

		{
			frame := NewFrame()
			frame.ReadFrom(srv.conn)

			assert.True(frame.Opcode().IsClose())
			assert.True(frame.IsMasked()) // client to server frames are masked
			frame.UnmaskPayload()

			closeCode, reason := DecodeCloseFramePayload(frame.Payload())
			assert.Equal(CloseProtocolError, closeCode)
			assert.Equal(reason, "")
		}
	}()
	time.Sleep(10 * time.Millisecond)

	ioc := sonic.MustIO()
	defer ioc.Close()

	ws, err := NewWebsocketStream(ioc, nil, RoleClient)
	if err != nil {
		t.Fatal(err)
	}

	done := false
	ws.AsyncHandshake("ws://localhost:8080", func(err error) {
		if err != nil {
			t.Fatal(err)
		}
		ws.AsyncNextFrame(func(err error, f Frame) {
			assert.Nil(err)
			assert.Equal(1, ws.Pending())
			ws.Flush()
			done = true
		})
	})

	for !done {
		ioc.PollOne()
	}
}

func TestClientEchoCloseCode(t *testing.T) {
	assert := assert.New(t)

	go func() {
		srv := &MockServer{}
		defer srv.Close()

		err := srv.Accept("localhost:8080")
		if err != nil {
			panic(err)
		}

		{
			frame := NewFrame()
			frame.
				SetFIN().
				SetClose().
				SetPayload(EncodeCloseFramePayload(CloseNormal, "something"))

			frame.WriteTo(srv.conn)
		}

		{
			frame := NewFrame()
			frame.ReadFrom(srv.conn)

			assert.True(frame.Opcode().IsClose())
			assert.True(frame.IsMasked()) // client to server frames are masked
			frame.UnmaskPayload()

			closeCode, reason := DecodeCloseFramePayload(frame.Payload())
			assert.Equal(CloseNormal, closeCode)
			assert.Equal(reason, "something")
		}
	}()
	time.Sleep(10 * time.Millisecond)

	ioc := sonic.MustIO()
	defer ioc.Close()

	ws, err := NewWebsocketStream(ioc, nil, RoleClient)
	if err != nil {
		t.Fatal(err)
	}

	done := false
	ws.AsyncHandshake("ws://localhost:8080", func(err error) {
		if err != nil {
			t.Fatal(err)
		}
		ws.AsyncNextFrame(func(err error, f Frame) {
			assert.Nil(err)
			assert.Equal(1, ws.Pending())
			ws.Flush()
			done = true
		})
	})

	for !done {
		ioc.PollOne()
	}
}

func TestClientSendPingWithInvalidPayload(t *testing.T) {
	// Per the protocol, pings cannot have payloads larger than 125. We send a ping with 125. The client should close
	// the connection immediately with 1002/Protocol Error.
	assert := assert.New(t)

	go func() {
		srv := &MockServer{}
		defer srv.Close()

		err := srv.Accept("localhost:8080")
		if err != nil {
			panic(err)
		}

		// This ping has an invalid payload size of 126, which should trigger a close with reason 1002.
		{
			frame := NewFrame()
			frame.
				SetFIN().
				SetPing().
				SetPayload(make([]byte, 126))
			assert.Equal(126, frame.PayloadLength())
			assert.Equal(2, frame.ExtendedPayloadLengthBytes())

			frame.WriteTo(srv.conn)
		}

		// Ensure we get the close.
		{
			frame := NewFrame()
			frame.ReadFrom(srv.conn)

			assert.True(frame.Opcode().IsClose())
			assert.True(frame.IsMasked()) // client to server frames are masked
			frame.UnmaskPayload()

			closeCode, reason := DecodeCloseFramePayload(frame.Payload())
			assert.Equal(CloseProtocolError, closeCode)
			assert.Empty(reason)
		}
	}()
	time.Sleep(10 * time.Millisecond)

	ioc := sonic.MustIO()
	defer ioc.Close()

	ws, err := NewWebsocketStream(ioc, nil, RoleClient)
	if err != nil {
		t.Fatal(err)
	}

	done := false
	ws.AsyncHandshake("ws://localhost:8080", func(err error) {
		if err != nil {
			t.Fatal(err)
		}
		ws.AsyncNextFrame(func(err error, f Frame) {
			assert.NotNil(err)
			assert.Equal(ErrControlFrameTooBig, err)
			assert.Equal(1, ws.Pending())
			ws.Flush()
			done = true
		})
	})

	for !done {
		ioc.PollOne()
	}
}

func TestClientSendMessageWithPayload126(t *testing.T) {
	assert := assert.New(t)

	go func() {
		srv := &MockServer{}
		defer srv.Close()

		err := srv.Accept("localhost:8080")
		if err != nil {
			panic(err)
		}

		frame := NewFrame()
		frame.
			SetFIN().
			SetText().
			SetPayload(make([]byte, 126))
		assert.Equal(126, frame.PayloadLength())
		assert.Equal(2, frame.ExtendedPayloadLengthBytes())

		frame.WriteTo(srv.conn)
	}()
	time.Sleep(10 * time.Millisecond)

	ioc := sonic.MustIO()
	defer ioc.Close()

	ws, err := NewWebsocketStream(ioc, nil, RoleClient)
	if err != nil {
		t.Fatal(err)
	}

	done := false
	ws.AsyncHandshake("ws://localhost:8080", func(err error) {
		if err != nil {
			t.Fatal(err)
		}
		ws.AsyncNextFrame(func(err error, f Frame) {
			if err != nil {
				t.Fatal(err)
			}
			assert.True(f.IsFIN())
			assert.True(f.Opcode().IsText())
			assert.Equal(126, f.PayloadLength())
			assert.Equal(2, f.ExtendedPayloadLengthBytes())
			done = true
		})
	})

	for !done {
		ioc.PollOne()
	}
}

func TestClientSendMessageWithPayload127(t *testing.T) {
	assert := assert.New(t)

	go func() {
		srv := &MockServer{}
		defer srv.Close()

		err := srv.Accept("localhost:8080")
		if err != nil {
			panic(err)
		}

		frame := NewFrame()
		frame.
			SetFIN().
			SetText().
			SetPayload(make([]byte, 1<<16+10 /* it won't fit in 2 bytes */))
		assert.Equal(1<<16+10, frame.PayloadLength())
		assert.Equal(8, frame.ExtendedPayloadLengthBytes())

		frame.WriteTo(srv.conn)
	}()
	time.Sleep(10 * time.Millisecond)

	ioc := sonic.MustIO()
	defer ioc.Close()

	ws, err := NewWebsocketStream(ioc, nil, RoleClient)
	if err != nil {
		t.Fatal(err)
	}

	done := false
	ws.AsyncHandshake("ws://localhost:8080", func(err error) {
		if err != nil {
			t.Fatal(err)
		}
		ws.AsyncNextFrame(func(err error, f Frame) {
			if err != nil {
				t.Fatal(err)
			}
			assert.True(f.IsFIN())
			assert.True(f.Opcode().IsText())
			assert.Equal(1<<16+10, f.PayloadLength())
			assert.Equal(8, f.ExtendedPayloadLengthBytes())
			done = true
		})
	})

	for !done {
		ioc.PollOne()
	}
}

func TestClientReconnectOnFailedRead(t *testing.T) {
	go func() {
		for i := 0; i < 10; i++ {
			srv := &MockServer{}

			err := srv.Accept("localhost:8080")
			if err != nil {
				panic(err)
			}

			srv.Write([]byte("hello"))
			srv.Close()
		}
	}()
	time.Sleep(10 * time.Millisecond)

	ioc := sonic.MustIO()
	defer ioc.Close()

	ws, err := NewWebsocketStream(ioc, &tls.Config{}, RoleClient)
	if err != nil {
		t.Fatal(err)
	}

	var (
		onHandshake   func(err error)
		onNextMessage func(err error, n int, mt MessageType)
		connect       func()
	)

	nread := 0

	b := make([]byte, 1024)
	onNextMessage = func(err error, n int, _ MessageType) {
		if err != nil {
			assertState(t, ws, StateTerminated)
			connect() // reconnect again
		} else {
			b = b[:n]
			if string(b) != "hello" {
				t.Fatal("expected hello")
			}

			nread++

			ws.AsyncNextMessage(b[:cap(b)], onNextMessage)
		}
	}

	done := false
	onHandshake = func(err error) {
		if err != nil {
			// could not reconnect
			done = true
			assertState(t, ws, StateTerminated)
		} else {
			ws.AsyncNextMessage(b, onNextMessage)
		}
	}

	connect = func() {
		ws.AsyncHandshake("ws://localhost:8080", onHandshake)
	}

	connect()

	for {
		if done {
			break
		}
		ioc.RunOne()
	}

	if nread != 10 {
		t.Fatal("should have read 10 times")
	}
}

func TestClientFailedHandshakeInvalidAddress(t *testing.T) {
	ioc := sonic.MustIO()
	defer ioc.Close()

	ws, err := NewWebsocketStream(ioc, nil, RoleClient)
	if err != nil {
		panic(err)
	}

	done := false
	ws.AsyncHandshake("localhost:8081", func(err error) {
		done = true
		if !errors.Is(err, ErrInvalidAddress) {
			t.Fatal("expected invalid address error")
		}

		assertState(t, ws, StateTerminated)
	})

	for {
		if done {
			break
		}
		ioc.RunOne()
	}

	assertState(t, ws, StateTerminated)
}

func TestClientFailedHandshakeNoServer(t *testing.T) {
	ioc := sonic.MustIO()
	defer ioc.Close()

	ws, err := NewWebsocketStream(ioc, nil, RoleClient)
	if err != nil {
		panic(err)
	}

	done := false
	ws.AsyncHandshake("ws://localhost:8081", func(err error) {
		done = true
		if err == nil {
			t.Fatal("expected error")
		}

		assertState(t, ws, StateTerminated)
	})

	for {
		if done {
			break
		}
		ioc.RunOne()
	}

	assertState(t, ws, StateTerminated)
}

func TestClientSuccessfulHandshake(t *testing.T) {
	srv := &MockServer{}

	go func() {
		defer srv.Close()

		err := srv.Accept("localhost:8080")
		if err != nil {
			panic(err)
		}
	}()
	time.Sleep(10 * time.Millisecond)

	ioc := sonic.MustIO()
	defer ioc.Close()

	ws, err := NewWebsocketStream(ioc, nil, RoleClient)
	if err != nil {
		t.Fatal(err)
	}

	var upgReqCbCalled, upgResCbCalled bool
	ws.SetUpgradeRequestCallback(func(req *http.Request) {
		upgReqCbCalled = true
		if val := req.Header.Get("Upgrade"); val != "websocket" {
			t.Fatalf("invalid Upgrade header in request: given=%s expected=%s", val, "websocket")
		}
	})
	ws.SetUpgradeResponseCallback(func(res *http.Response) {
		upgResCbCalled = true
		if val := res.Header.Get("Upgrade"); val != "websocket" {
			t.Fatalf("invalid Upgrade header in response: given=%s expected=%s", val, "websocket")
		}
	})

	assertState(t, ws, StateHandshake)

	ws.AsyncHandshake("ws://localhost:8080", func(err error) {
		if err != nil {
			assertState(t, ws, StateTerminated)
		} else {
			assertState(t, ws, StateActive)
			if !upgReqCbCalled {
				t.Fatal("upgrade request callback not invoked")
			}
			if !upgResCbCalled {
				t.Fatal("upgrade response callback not invoked")
			}
		}
	})

	for {
		if srv.IsClosed() {
			break
		}
		ioc.RunOne()
	}
}

func TestClientSuccessfulHandshakeWithExtraHeaders(t *testing.T) {
	srv := &MockServer{}

	go func() {
		defer srv.Close()

		err := srv.Accept("localhost:8080")
		if err != nil {
			panic(err)
		}
	}()
	time.Sleep(10 * time.Millisecond)

	ioc := sonic.MustIO()
	defer ioc.Close()

	ws, err := NewWebsocketStream(ioc, nil, RoleClient)
	if err != nil {
		t.Fatal(err)
	}

	assertState(t, ws, StateHandshake)

	// Keys are automatically canonicalized by Go's protocol implementation -
	// hence we don't care about their casing here.
	expected := map[string][]string{
		"k1": {"v1"},
		"k2": {"v21", "v22"},
		"k3": {"v32"},
		"k4": {"v4"},
		"k5": {"v51", "v52"},
		"k6": {"v62"},
	}

	ws.AsyncHandshake(
		"ws://localhost:8080",
		func(err error) {
			if err != nil {
				assertState(t, ws, StateTerminated)
			} else {
				assertState(t, ws, StateActive)
			}
		},
		ExtraHeader(true, "k1", "v1"),
		ExtraHeader(true, "k2", "v21", "v22"),
		ExtraHeader(true, "k3", "v31"), ExtraHeader(true, "k3", "v32"),
		ExtraHeader(false, "k4", "v4"),
		ExtraHeader(false, "k5", "v51", "v52"),
		ExtraHeader(false, "k6", "v61"), ExtraHeader(false, "k6", "v62"),
	)

	for !srv.IsClosed() {
		ioc.RunOne()
	}

	for key := range expected {
		given := srv.Upgrade.Header.Values(key)
		if len(given) != len(expected[key]) {
			t.Fatal("wrong extra header")
		}
		for i := 0; i < len(given); i++ {
			if given[i] != expected[key][i] {
				t.Fatal("wrong extra header")
			}
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

	assertState(t, ws, StateActive)
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

	assertState(t, ws, StateActive)
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

	assertState(t, ws, StateActive)
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

	assertState(t, ws, StateActive)
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

	closeFrame := ws.pendingFrames[0]
	closeFrame.UnmaskPayload()

	cc, _ := DecodeCloseFramePayload(ws.pendingFrames[0].Payload())
	if cc != CloseProtocolError {
		t.Fatal("should have closed with protocol error")
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

		closeFrame := ws.pendingFrames[0]
		closeFrame.UnmaskPayload()

		cc, _ := DecodeCloseFramePayload(ws.pendingFrames[0].Payload())
		if cc != CloseProtocolError {
			t.Fatal("should have closed with protocol error")
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
	mock := NewMockStream()
	ws.init(mock)

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

		reply := ws.pendingFrames[0]
		if !(reply.Opcode().IsPong() && reply.IsMasked()) {
			t.Fatal("invalid pong reply")
		}

		reply.UnmaskPayload()
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

func TestClientAsyncReadPingFrame(t *testing.T) {
	ioc := sonic.MustIO()
	defer ioc.Close()

	ws, err := NewWebsocketStream(ioc, nil, RoleClient)
	if err != nil {
		t.Fatal(err)
	}

	ws.state = StateActive
	mock := NewMockStream()
	ws.init(mock)

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

		reply := ws.pendingFrames[0]
		if !(reply.Opcode().IsPong() && reply.IsMasked()) {
			t.Fatal("invalid pong reply")
		}

		reply.UnmaskPayload()
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

func TestClientReadPongFrame(t *testing.T) {
	ioc := sonic.MustIO()
	defer ioc.Close()

	ws, err := NewWebsocketStream(ioc, nil, RoleClient)
	if err != nil {
		t.Fatal(err)
	}

	ws.state = StateActive
	mock := NewMockStream()
	ws.init(mock)

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

func TestClientAsyncReadPongFrame(t *testing.T) {
	ioc := sonic.MustIO()
	defer ioc.Close()

	ws, err := NewWebsocketStream(ioc, nil, RoleClient)
	if err != nil {
		t.Fatal(err)
	}

	ws.state = StateActive
	mock := NewMockStream()
	ws.init(mock)

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

func TestClientReadCloseFrame(t *testing.T) {
	ioc := sonic.MustIO()
	defer ioc.Close()

	ws, err := NewWebsocketStream(ioc, nil, RoleClient)
	if err != nil {
		t.Fatal(err)
	}

	ws.state = StateActive
	mock := NewMockStream()
	ws.init(mock)

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

		reply := ws.pendingFrames[0]
		if !reply.IsMasked() {
			t.Fatal("reply should be masked")
		}
		reply.UnmaskPayload()

		cc, reason := DecodeCloseFramePayload(reply.Payload())
		if !(cc == CloseNormal && reason == "bye") {
			t.Fatal("invalid close frame reply")
		}

		assertState(t, ws, StateClosedByPeer)
	})

	b := make([]byte, 128)
	_, _, err = ws.NextMessage(b)

	if len(ws.pendingFrames) > 0 {
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

func TestClientAsyncReadCloseFrame(t *testing.T) {
	ioc := sonic.MustIO()
	defer ioc.Close()

	ws, err := NewWebsocketStream(ioc, nil, RoleClient)
	if err != nil {
		t.Fatal(err)
	}

	ws.state = StateActive
	mock := NewMockStream()
	ws.init(mock)

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

		reply := ws.pendingFrames[0]
		if !reply.IsMasked() {
			t.Fatal("reply should be masked")
		}
		reply.UnmaskPayload()

		cc, reason := DecodeCloseFramePayload(reply.Payload())
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

func TestClientWriteFrame(t *testing.T) {
	ioc := sonic.MustIO()
	defer ioc.Close()

	ws, err := NewWebsocketStream(ioc, nil, RoleClient)
	if err != nil {
		t.Fatal(err)
	}

	mock := NewMockStream()
	ws.state = StateActive
	ws.init(mock)

	f := ws.AcquireFrame()
	f.SetFIN()
	f.SetText()
	f.SetPayload([]byte{1, 2, 3, 4, 5})

	if len(*f) != 2 /*mandatory header*/ +4 /*mask since it's written by a client*/ +5 /*payload length*/ {
		t.Fatal("invalid frame length")
	}

	err = ws.WriteFrame(f)
	if err != nil {
		t.Fatal(err)
	} else {
		mock.b.Commit(mock.b.WriteLen())

		f := Frame(make([]byte, 2+4+5))

		_, err = f.ReadFrom(mock.b)
		if err != nil {
			t.Fatal(err)
		}

		if !(f.IsFIN() && f.IsMasked() && f.Opcode().IsText()) {
			t.Fatal("frame is corrupt, something went wrong with the encoder")
		}

		f.UnmaskPayload()

		if !bytes.Equal(f.Payload(), []byte{1, 2, 3, 4, 5}) {
			t.Fatal("frame payload is corrupt, something went wrong with the encoder")
		}

		assertState(t, ws, StateActive)
	}
}

func TestClientAsyncWriteFrame(t *testing.T) {
	ioc := sonic.MustIO()
	defer ioc.Close()

	ws, err := NewWebsocketStream(ioc, nil, RoleClient)
	if err != nil {
		t.Fatal(err)
	}

	mock := NewMockStream()
	ws.state = StateActive
	ws.init(mock)

	f := ws.AcquireFrame()
	f.SetFIN()
	f.SetText()
	f.SetPayload([]byte{1, 2, 3, 4, 5})

	ran := false

	ws.AsyncWriteFrame(f, func(err error) {
		ran = true

		if err != nil {
			t.Fatal(err)
		} else {
			mock.b.Commit(mock.b.WriteLen())

			f := NewFrame()

			_, err = f.ReadFrom(mock.b)
			if err != nil {
				t.Fatal(err)
			}

			if !(f.IsFIN() && f.IsMasked() && f.Opcode().IsText()) {
				t.Fatal("frame is corrupt, something went wrong with the encoder")
			}

			f.UnmaskPayload()

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

func TestClientWrite(t *testing.T) {
	ioc := sonic.MustIO()
	defer ioc.Close()

	ws, err := NewWebsocketStream(ioc, nil, RoleClient)
	if err != nil {
		t.Fatal(err)
	}

	mock := NewMockStream()
	ws.state = StateActive
	ws.init(mock)

	err = ws.Write([]byte{1, 2, 3, 4, 5}, TypeText)
	if err != nil {
		t.Fatal(err)
	} else {
		mock.b.Commit(mock.b.WriteLen())

		f := NewFrame()

		_, err = f.ReadFrom(mock.b)
		if err != nil {
			t.Fatal(err)
		}

		if !(f.IsFIN() && f.IsMasked() && f.Opcode().IsText()) {
			t.Fatal("frame is corrupt, something went wrong with the encoder")
		}

		f.UnmaskPayload()

		if !bytes.Equal(f.Payload(), []byte{1, 2, 3, 4, 5}) {
			t.Fatal("frame payload is corrupt, something went wrong with the encoder")
		}

		assertState(t, ws, StateActive)
	}
}

func TestClientAsyncWrite(t *testing.T) {
	ioc := sonic.MustIO()
	defer ioc.Close()

	ws, err := NewWebsocketStream(ioc, nil, RoleClient)
	if err != nil {
		t.Fatal(err)
	}

	mock := NewMockStream()
	ws.state = StateActive
	ws.init(mock)

	ws.AsyncWrite([]byte{1, 2, 3, 4, 5}, TypeText, func(err error) {
		if err != nil {
			t.Fatal(err)
		} else {
			mock.b.Commit(mock.b.WriteLen())

			f := NewFrame()
			_, err = f.ReadFrom(mock.b)
			if err != nil {
				t.Fatal(err)
			}

			if !(f.IsFIN() && f.IsMasked() && f.Opcode().IsText()) {
				t.Fatal("frame is corrupt, something went wrong with the encoder")
			}

			f.UnmaskPayload()

			if !bytes.Equal(f.Payload(), []byte{1, 2, 3, 4, 5}) {
				t.Fatal("frame payload is corrupt, something went wrong with the encoder")
			}

			assertState(t, ws, StateActive)
		}
	})
}

func TestClientClose(t *testing.T) {
	ioc := sonic.MustIO()
	defer ioc.Close()

	ws, err := NewWebsocketStream(ioc, nil, RoleClient)
	if err != nil {
		t.Fatal(err)
	}

	mock := NewMockStream()
	ws.state = StateActive
	ws.init(mock)

	err = ws.Close(CloseNormal, "bye")
	if err != nil {
		t.Fatal(err)
	} else {
		mock.b.Commit(mock.b.WriteLen())

		f := NewFrame()
		_, err = f.ReadFrom(mock.b)
		if err != nil {
			t.Fatal(err)
		}

		if !(f.IsFIN() && f.IsMasked() && f.Opcode().IsClose()) {
			t.Fatal("frame is corrupt, something went wrong with the encoder")
		}

		f.UnmaskPayload()

		if f.PayloadLength() != 5 {
			t.Fatal("wrong message in close frame")
		}

		cc, reason := DecodeCloseFramePayload(f.Payload())
		if !(cc == CloseNormal && reason == "bye") {
			t.Fatal("wrong close frame payload")
		}
	}
}

func TestClientAsyncClose(t *testing.T) {
	ioc := sonic.MustIO()
	defer ioc.Close()

	ws, err := NewWebsocketStream(ioc, nil, RoleClient)
	if err != nil {
		t.Fatal(err)
	}

	mock := NewMockStream()
	ws.state = StateActive
	ws.init(mock)

	ran := false
	ws.AsyncClose(CloseNormal, "bye", func(err error) {
		ran = true

		if err != nil {
			t.Fatal(err)
		} else {
			mock.b.Commit(mock.b.WriteLen())

			f := NewFrame()
			_, err = f.ReadFrom(mock.b)
			if err != nil {
				t.Fatal(err)
			}

			if !(f.IsFIN() && f.IsMasked() && f.Opcode().IsClose()) {
				t.Fatal("frame is corrupt, something went wrong with the encoder")
			}

			f.UnmaskPayload()

			if f.PayloadLength() != 5 {
				t.Fatal("wrong message in close frame")
			}

			cc, reason := DecodeCloseFramePayload(f.Payload())
			if !(cc == CloseNormal && reason == "bye") {
				t.Fatal("wrong close frame payload")
			}
		}
	})

	if !ran {
		t.Fatal("async read did not run")
	}
}

func TestClientCloseHandshakeWeStart(t *testing.T) {
	ioc := sonic.MustIO()
	defer ioc.Close()

	ws, err := NewWebsocketStream(ioc, nil, RoleClient)
	if err != nil {
		t.Fatal(err)
	}

	mock := NewMockStream()
	ws.state = StateActive
	ws.init(mock)

	err = ws.Close(CloseNormal, "bye")
	if err != nil {
		t.Fatal(err)
	} else {
		assertState(t, ws, StateClosedByUs)

		mock.b.Commit(mock.b.WriteLen())

		serverReply := NewFrame()
		serverReply.SetFIN()
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

		if !(reply.IsFIN() && reply.Opcode().IsClose()) {
			t.Fatal("wrong close reply")
		}

		cc, reason := DecodeCloseFramePayload(reply.Payload())
		if !(cc == CloseNormal && reason == "bye") {
			t.Fatal("wrong close frame payload reply")
		}

		assertState(t, ws, StateCloseAcked)
	}
}

func TestClientAsyncCloseHandshakeWeStart(t *testing.T) {
	ioc := sonic.MustIO()
	defer ioc.Close()

	ws, err := NewWebsocketStream(ioc, nil, RoleClient)
	if err != nil {
		t.Fatal(err)
	}

	mock := NewMockStream()
	ws.state = StateActive
	ws.init(mock)

	ran := false
	ws.AsyncClose(CloseNormal, "bye", func(err error) {
		ran = true

		if err != nil {
			t.Fatal(err)
		} else {
			mock.b.Commit(mock.b.WriteLen())

			serverReply := NewFrame()
			serverReply.SetFIN()
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

			if !(reply.IsFIN() && reply.Opcode().IsClose()) {
				t.Fatal("wrong close reply")
			}

			cc, reason := DecodeCloseFramePayload(reply.Payload())
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

func TestClientCloseHandshakePeerStarts(t *testing.T) {
	ioc := sonic.MustIO()
	defer ioc.Close()

	ws, err := NewWebsocketStream(ioc, nil, RoleClient)
	if err != nil {
		t.Fatal(err)
	}

	mock := NewMockStream()
	ws.state = StateActive
	ws.init(mock)

	serverClose := NewFrame()
	serverClose.SetFIN()
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

	if !recv.Opcode().IsClose() {
		t.Fatal("should have received close")
	}

	assertState(t, ws, StateClosedByPeer)

	cc, reason := DecodeCloseFramePayload(recv.Payload())
	if !(cc == CloseNormal && reason == "bye") {
		t.Fatal("peer close frame payload is corrupt")
	}

	if ws.Pending() != 1 {
		t.Fatal("should have a pending reply ready to be flushed")
	}
}

func TestClientAsyncCloseHandshakePeerStarts(t *testing.T) {
	ioc := sonic.MustIO()
	defer ioc.Close()

	ws, err := NewWebsocketStream(ioc, nil, RoleClient)
	if err != nil {
		t.Fatal(err)
	}

	mock := NewMockStream()
	ws.state = StateActive
	ws.init(mock)

	serverClose := NewFrame()
	serverClose.SetFIN()
	serverClose.SetClose()
	serverClose.SetPayload(EncodeCloseFramePayload(CloseNormal, "bye"))

	nn, err := serverClose.WriteTo(ws.src)
	if err != nil {
		t.Fatal(err)
	}

	ws.src.Commit(int(nn))

	ws.AsyncNextFrame(func(err error, recv Frame) {
		if err != nil {
			t.Fatal(err)
		}

		if !recv.Opcode().IsClose() {
			t.Fatal("should have received close")
		}

		assertState(t, ws, StateClosedByPeer)

		cc, reason := DecodeCloseFramePayload(recv.Payload())
		if !(cc == CloseNormal && reason == "bye") {
			t.Fatal("peer close frame payload is corrupt")
		}

		if ws.Pending() != 1 {
			t.Fatal("should have a pending reply ready to be flushed")
		}
	})
}
