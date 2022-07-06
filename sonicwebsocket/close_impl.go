package sonicwebsocket

import (
	"encoding/binary"
	"fmt"
	"io"

	"github.com/talostrading/sonic"
)

func (s *WebsocketStream) AsyncClose(cc CloseCode, reason string, cb func(err error)) {
	switch s.state {
	case StateHandshake:
		cb(ErrCloseWhileHandshaking)
	case StateOpen:
		s.asyncClose(cc, reason, cb)
	case StateClosing:
		// Either we called a close function before or we received a close frame and
		// we are in the process of replying. In both cases, we do not need to do anything here.
	case StateClosed:
		cb(nil)
	}
}

func (s *WebsocketStream) Close(cc CloseCode, reason string) error {
	panic("Close(...) not yet supported. Use AsyncClose(...) instead.")
}

func (s *WebsocketStream) asyncClose(cc CloseCode, reason string, cb func(err error)) {
	b := make([]byte, DefaultPayloadSize)
	s.asyncReadPending(b, func(err error, n int) {
		if err != nil {
			cb(err)
		} else {
			switch s.state {
			case StateHandshake:
				// Not possible.
			case StateOpen:
				s.asyncCloseNow(cc, reason, cb)
			case StateClosing:
				// Will turn into StateClosed on the next close call.
			case StateClosed:
				// The pending read was a close frame from the other peer.
				cb(nil)
			}
		}
	})

}

func (s *WebsocketStream) asyncCloseNow(cc CloseCode, reason string, cb func(err error)) {
	s.state = StateClosing

	payload := make([]byte, 2)
	binary.BigEndian.PutUint16(payload, uint16(cc))
	payload = append(payload, []byte(reason)...)

	s.AsyncWriteSome(true, payload, func(err error, n int) {
		if err != nil {
			cb(ErrOnClose)
		} else {
			b := make([]byte, 128)
			s.AsyncRead(b, func(err error, n int) {
				if err != nil {
					err = ErrOnClose
				} else {
				}
				cb(err)
			})
		}
	})
}

func (s *WebsocketStream) asyncReadPending(b []byte, cb sonic.AsyncCallback) {
	fmt.Println("reading pending")
	s.AsyncRead(b, func(err error, n int) {
		if err != nil {
			if err != io.EOF {
				err = ErrOnClose
			}
			n = 0
		}
		cb(nil, n)
	})
}
