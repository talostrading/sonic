package sonicwebsocket

import (
	"bytes"
	"crypto/sha1"
	"crypto/tls"
	"hash"

	"github.com/talostrading/sonic"
	"github.com/talostrading/sonic/sonicwebsocket/internal"
)

var _ Stream = &WebsocketStream{}

// WebsocketStream is a stateful full-duplex connection between two endpoints which
// adheres to the WebSocket protocol.
//
// The WebsocketStream implements Stream and can be used by both clients and servers.
//
// The underlying socket through which all IO is done is and must remain in blocking mode.
type WebsocketStream struct {
	ioc    *sonic.IO            // executes async operations for WebsocketStream
	state  StreamState          // current stream state
	tls    *tls.Config          // the optional TLS config used when wanting to connect to `wss` scheme endpoints
	ccb    AsyncControlCallback // the callback invoked when a control frame is received
	stream sonic.Stream         // the stream through which websocket data is sent/received
	role   Role                 // role of the stream: client or server

	// closeTimer is a timer which closes the underlying stream on expiry in the client role.
	// The expected behaviour is for the server to close the connection such that the client receives an io.EOF.
	// If that does not happen, this timer does it for the client.
	closeTimer *sonic.Timer

	// --------- used when reading ---------
	rbuf    []byte              // holds unparsed data as read from the wire
	ropcode Opcode              // currently read message opcode: binary or text
	rcont   bool                // true if the next frame is a continuation
	rdone   bool                // set when a message is done
	rclose  bool                // set when we read a close frame
	rblock  *internal.SoftMutex // locked if we are currently reading
	rframe  *frame              // data frame in which read data is deserialized into
	rmax    uint64              // max message size

	// --------- used when writing ---------
	// TODO i fucking hate this bytes.Buffer do something like a boost flat_buffer
	wbuf    *bytes.Buffer       // holds the frames which will be written on the wire
	wopcode Opcode              // currently written opcode: binary or text
	wframe  *frame              // data frame in which data is read from the wire
	wblock  *internal.SoftMutex // locked is we are currently writing

	// --------- used when handshaking ---------
	hasher hash.Hash // hashes Sec-Websocket-Key when the stream is a client

	// TODO remove this
	controlFramePayload []byte
}

func NewWebsocketStream(ioc *sonic.IO, tls *tls.Config, role Role) (Stream, error) {
	closeTimer, err := sonic.NewTimer(ioc)
	if err != nil {
		return nil, err
	}

	s := &WebsocketStream{
		ioc:        ioc,
		state:      StateClosed,
		tls:        tls,
		role:       role,
		closeTimer: closeTimer,

		// used when reading
		rbuf:    make([]byte, MaxPending),
		ropcode: OpcodeText,
		rcont:   false,
		rdone:   false,
		rclose:  false,
		rblock:  &internal.SoftMutex{},
		rframe:  newFrame(),
		rmax:    MaxMessageSize,

		// used when writing
		wbuf:    bytes.NewBuffer(make([]byte, 0, DefaultFrameSize)),
		wopcode: OpcodeText,
		wframe:  newFrame(),
		wblock:  &internal.SoftMutex{},

		// used when handshaking
		hasher: sha1.New(),

		controlFramePayload: make([]byte, MaxControlFramePayloadSize),
	}

	return s, nil
}

func (s *WebsocketStream) Flush() {

}

func (s *WebsocketStream) DeflateSupported() bool {
	return false
}

func (s *WebsocketStream) NextLayer() sonic.Stream {
	return s.stream
}

// SetReadLimit sets the maximum read size. If 0, the max size is used.
func (s *WebsocketStream) SetMaxMessageSize(limit uint64) {
	if limit == 0 {
		s.rmax = MaxMessageSize
	} else {
		s.rmax = limit
	}
}

func (s *WebsocketStream) State() StreamState {
	return s.state
}

func (s *WebsocketStream) GotText() bool {
	return s.rframe.IsText()
}

func (s *WebsocketStream) GotBinary() bool {
	return s.rframe.IsBinary()
}

func (s *WebsocketStream) IsMessageDone() bool {
	return s.rframe.IsFin()
}

func (s *WebsocketStream) SentBinary() bool {
	return s.wopcode == OpcodeBinary
}

func (s *WebsocketStream) SentText() bool {
	return s.wopcode == OpcodeText
}

func (s *WebsocketStream) SendText(v bool) {
	s.wopcode = OpcodeText
}

func (s *WebsocketStream) SendBinary(v bool) {
	s.wopcode = OpcodeBinary
}

func (s *WebsocketStream) SetControlCallback(ccb AsyncControlCallback) {
	s.ccb = ccb
}

func (s *WebsocketStream) ControlCallback() AsyncControlCallback {
	return s.ccb
}

func (s *WebsocketStream) Closed() bool {
	return s.state == StateClosed
}
