// In this example we connect to the Binance Spot market to fetch the latest books for
// BNB-BTC. We first connect to TLS websocket endpoint, then send a subscription message
// after which we expect to read a bunch of books from the socket.
//
// We also set a timer to periodically ping the websocket peer, per Binance's spec. We
// put a timestamp in the payload which will then be returned on a pong from Binance.
// Per the websocket spec, the pong should be sent immediately after a ping is received.
// Taking a timestamp and subtracting from it the one in the pong's payload thus gives us
// an idea of the application layer RTT.
package main

import (
	"crypto/tls"
	"encoding/binary"
	"fmt"
	"log"
	"time"

	"github.com/talostrading/sonic"
	"github.com/talostrading/sonic/codec/websocket"
	"github.com/talostrading/sonic/util"
)

type Stream struct {
	ioc       *sonic.IO
	stream    websocket.Stream
	pingTimer *sonic.Timer
	b         [4096]byte
}

func NewStream(ioc *sonic.IO) (s *Stream, err error) {
	s = &Stream{
		ioc: ioc,
	}

	s.stream, err = websocket.NewWebsocketStream(ioc, &tls.Config{}, websocket.RoleClient)
	if err != nil {
		return nil, err
	}

	s.pingTimer, err = sonic.NewTimer(ioc)
	if err != nil {
		return nil, err
	}

	err = s.pingTimer.ScheduleRepeating(5*time.Second, func() {
		if s.stream.State() == websocket.StateActive {
			binary.LittleEndian.PutUint64(s.b[:8], uint64(util.GetMonoTimeNanos()))
			s.stream.AsyncWrite(s.b[:8], websocket.TypePing, func(err error) {
				if err != nil {
					s.forceClose(err)
				}
			})
		}
	})
	if err != nil {
		return nil, err
	}

	s.stream.SetControlCallback(s.onControl)

	return s, nil
}

func (s *Stream) onControl(t websocket.MessageType, payload []byte) {
	switch t {
	case websocket.TypePong:
		t0 := int64(binary.LittleEndian.Uint64(payload[:8]))
		t1 := util.GetMonoTimeNanos()
		log.Println("RTT", time.Duration(t1-t0)*time.Nanosecond)
	case websocket.TypeClose:
		log.Println("Connection closed, reconnecting...")
		s.Setup()
	default:
		s.forceClose(fmt.Errorf("unexpected control message %s", t))
	}
}

func (s *Stream) forceClose(err error) {
	if s.pingTimer != nil {
		s.pingTimer.Close()
	}
	if s.stream != nil {
		s.stream.CloseNextLayer() // closes the underlying TCP connection
	}
	s.ioc.Close() // at this point the event loop will break

	log.Fatal(err)
}

func (s *Stream) Setup() {
	s.stream.AsyncHandshake("wss://stream.binance.com:9443/ws", s.onHandshake)
}

func (s *Stream) onHandshake(err error) {
	if err != nil {
		s.forceClose(err)
	} else {
		s.stream.AsyncWrite(
			[]byte(`{"id":1,"method":"SUBSCRIBE","params":["bnbbtc@depth"]}`),
			websocket.TypeText,
			s.onSubscribe,
		)
	}
}

func (s *Stream) onSubscribe(err error) {
	if err != nil {
		s.forceClose(err)
	} else {
		s.asyncRead()
	}
}

func (s *Stream) asyncRead() {
	s.stream.AsyncNextMessage(s.b[:], s.onRead)
}

// onRead only received Text or Binary messages. In this case, only Text as per Binance's spec.
func (s *Stream) onRead(err error, n int, t websocket.MessageType) {
	if err != nil {
		s.forceClose(err)
	} else {
		log.Println("payload", string(s.b[:n]))
		s.asyncRead()
	}
}

func main() {
	ioc := sonic.MustIO()
	defer ioc.Close()

	stream, err := NewStream(ioc)
	if err != nil {
		log.Fatal(err)
	}

	stream.Setup()

	ioc.Run()
}
