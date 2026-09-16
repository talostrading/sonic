package websocket

import (
	"bytes"
	"fmt"
	"net"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/talostrading/sonic"
)

func runHandshake(
	t *testing.T,
	configure func(*MockServer),
) (ws *Stream, ioc *sonic.IO, handshakeErr error, cleanup func()) {
	t.Helper()

	srv := NewMockServer()
	configure(srv)

	go func() {
		defer srv.Close()
		_ = srv.Accept(MockServerDynamicAddr)
	}()

	ioc = sonic.MustIO()

	ws, err := NewWebsocketStream(ioc, nil, RoleClient)
	assert.Nil(t, err)

	done := false
	ws.AsyncHandshake(
		fmt.Sprintf("ws://127.0.0.1:%d", <-srv.portChan),
		func(err error) {
			handshakeErr = err
			done = true
		},
	)
	for !done {
		ioc.PollOne()
	}

	return ws, ioc, handshakeErr, func() {
		srv.Close()
		ioc.Close()
	}
}

func TestClientHandshakeResponseSplitAcrossReads(t *testing.T) {
	for _, tc := range []struct {
		name string
		cuts func(res []byte) []int
	}{
		{
			"after the status line",
			func(res []byte) []int { return []int{10} },
		},
		{
			"inside the headers",
			func(res []byte) []int { return []int{len(res) / 2} },
		},
		{
			"inside the terminator",
			func(res []byte) []int {
				return []int{len(res) - 3, len(res) - 2, len(res) - 1}
			},
		},
		{
			"every 16 bytes",
			func(res []byte) (cuts []int) {
				for i := 16; i < len(res); i += 16 {
					cuts = append(cuts, i)
				}
				return cuts
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err, cleanup := runHandshake(t, func(srv *MockServer) {
				srv.SendUpgradeResponse = func(conn net.Conn, res []byte) error {
					prev := 0
					for _, cut := range append(tc.cuts(res), len(res)) {
						if _, err := conn.Write(res[prev:cut]); err != nil {
							return err
						}
						time.Sleep(5 * time.Millisecond)
						prev = cut
					}
					return nil
				}
			})
			defer cleanup()

			assert.Nil(t, err)
		})
	}
}

func TestClientHandshakeLargeResponse(t *testing.T) {
	_, _, err, cleanup := runHandshake(t, func(srv *MockServer) {
		srv.SendUpgradeResponse = func(conn net.Conn, res []byte) error {
			cookie := "Set-Cookie: session=" + strings.Repeat("x", 8192) + "\r\n"
			big := append([]byte{}, res[:len(res)-2]...)
			big = append(big, cookie...)
			big = append(big, "\r\n"...)
			_, err := conn.Write(big)
			return err
		}
	})
	defer cleanup()

	assert.Nil(t, err)
}

func TestClientHandshakeFramesBehindNonCanonicalResponse(t *testing.T) {
	payload := []byte("first frame of the session")

	ws, ioc, err, cleanup := runHandshake(t, func(srv *MockServer) {
		srv.SendUpgradeResponse = func(conn net.Conn, _ []byte) error {
			key := MakeResponseKey(
				[]byte(srv.Upgrade.Header.Get("Sec-WebSocket-Key")),
			)

			res := bytes.NewBuffer(nil)
			fmt.Fprintf(res, "HTTP/1.1 101 Switching Protocols\r\n")
			fmt.Fprintf(res, "upgrade:  websocket\r\n")
			fmt.Fprintf(res, "connection:  Upgrade\r\n")
			fmt.Fprintf(res, "sec-websocket-accept:  %s\r\n", key)
			fmt.Fprintf(res, "\r\n")

			frame := NewFrame()
			frame.SetFIN()
			frame.SetText()
			frame.SetPayload(payload)
			if _, err := frame.WriteTo(res); err != nil {
				return err
			}

			_, err := res.WriteTo(conn)
			return err
		}
	})
	defer cleanup()

	assert.Nil(t, err)

	b := make([]byte, 128)
	done := false
	ws.AsyncNextMessage(b, func(err error, n int, mt MessageType) {
		assert.Nil(t, err)
		assert.Equal(t, payload, b[:n])
		done = true
	})
	for !done {
		ioc.PollOne()
	}
}

func TestClientHandshakeStalledResponse(t *testing.T) {
	start := time.Now()

	_, _, err, cleanup := runHandshake(t, func(srv *MockServer) {
		srv.SendUpgradeResponse = func(conn net.Conn, res []byte) error {
			cut := bytes.Index(res, []byte("\r\n")) + 2
			if _, err := conn.Write(res[:cut]); err != nil {
				return err
			}
			time.Sleep(HandshakeReadTimeout + 2*time.Second)
			return nil
		}
	})
	defer cleanup()

	assert.ErrorIs(t, err, os.ErrDeadlineExceeded)
	assert.Less(t, time.Since(start), HandshakeReadTimeout+time.Second)
}
