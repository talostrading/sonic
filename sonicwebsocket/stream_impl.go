package sonicwebsocket

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"crypto/sha1"
	"crypto/tls"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"hash"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"syscall"

	"github.com/talostrading/sonic"
	"github.com/talostrading/sonic/util"
)

var _ Stream = &WebsocketStream{}

// WebsocketStream is a stateful full-duplex connection between two endpoints which
// adheres to the WebSocket protocol.
//
// The WebsocketStream implements Stream and can be used by both clients and servers.
//
// The underlying socket through which all IO is done is and must remain in blocking mode.
type WebsocketStream struct {
	ioc    *sonic.IO    // executes async operations for WebsocketStream
	state  StreamState  // current stream state
	tls    *tls.Config  // the optional TLS config used when wanting to connect to `wss` scheme endpoints
	stream sonic.Stream // the stream through which websocket data is sent/received
	role   Role         // role of the stream: client or server

	// closeTimer is a timer which closes the underlying stream on expiry in the client role.
	// The expected behaviour is for the server to close the connection such that the client receives an io.EOF.
	// If that does not happen, this timer does it for the client.
	closeTimer *sonic.Timer

	// Contains the currently read frame.
	//
	// The payload is empty a data frame is read. In this case, the payload is
	// directly filled into the caller's buffer to avoid any copying.
	//
	// The payload may not be empty if a control frame is read.
	readFrame *Frame

	// Read buffer which contains the unparsed frames as received from the wire.
	//
	// We try to read as much as possible into the caller's buffer, however there are cases
	// when the peer sends data immediately after the handshake and before the caller
	// schedules any async read. In this instance, the data succeeding the handshake is read into
	// rd and parsed on the next read. Note that rd might contain incomplete frames as a result of short reads.
	rb []byte        // read buffer which contains unparsed frames as received from the wire
	rd *bytes.Reader // reader used to read the above buffer

	// Frames that are waiting to be written on the wire.
	pendingWrite []*Frame

	// Used to hash the handshake key.
	hasher hash.Hash // hashes Sec-Websocket-Key when the stream is a client
}

func NewWebsocketStream(ioc *sonic.IO, tls *tls.Config, role Role) (Stream, error) {
	closeTimer, err := sonic.NewTimer(ioc)
	if err != nil {
		return nil, err
	}

	s := &WebsocketStream{
		ioc:        ioc,
		state:      StateTerminated,
		tls:        tls,
		role:       role,
		closeTimer: closeTimer,

		readFrame: newFrame(),

		rb:           make([]byte, MaxPending),
		pendingWrite: make([]*Frame, 0, 128),

		hasher: sha1.New(),
	}
	s.rd = bytes.NewReader(s.rb)

	return s, nil
}

func (s *WebsocketStream) Flush() error {
	for i := range s.pendingWrite {
		if err := s.WriteFrame(s.pendingWrite[i]); err != nil && err == io.EOF {
			return err
		}
	}
	s.pendingWrite = s.pendingWrite[:0]
	return nil
}

func (s *WebsocketStream) AsyncFlush(cb func(err error)) {
	s.asyncFlush(0, cb)
}

func (s *WebsocketStream) asyncFlush(ix int, cb func(err error)) {
	if ix < len(s.pendingWrite) {
		s.AsyncWriteFrame(s.pendingWrite[ix], func(err error) {
			if err != nil && err == io.EOF {
				cb(err)
			} else {
				ReleaseFrame(s.pendingWrite[ix])
				s.asyncFlush(ix+1, cb)
			}
		})
	} else {
		s.pendingWrite = s.pendingWrite[:0]

		cb(nil)
	}
}

func (s *WebsocketStream) DeflateSupported() bool {
	return false
}

func (s *WebsocketStream) NextLayer() sonic.Stream {
	return s.stream
}

func (s *WebsocketStream) State() StreamState {
	return s.state
}

func (s *WebsocketStream) Accept() error {
	if s.role != RoleServer {
		return fmt.Errorf("invalid role=%s; only servers can accept", s.role)
	}

	// TODO
	return nil
}

func (s *WebsocketStream) AsyncAccept(cb func(error)) {
	if s.role != RoleServer {
		cb(fmt.Errorf("invalid role=%s; only servers can accept", s.role))
		return
	}
	// TODO
}

func (s *WebsocketStream) Handshake(addr string) (err error) {
	if s.role != RoleClient {
		return fmt.Errorf("invalid role=%s; only clients can handshake", s.role)
	}

	done := make(chan struct{}, 1)
	s.handshake(addr, func(herr error) {
		done <- struct{}{}
		err = herr
	})
	<-done
	return
}

func (s *WebsocketStream) AsyncHandshake(addr string, cb func(error)) {
	if s.role != RoleClient {
		cb(fmt.Errorf("invalid role=%s; only clients can establis", s.role))
		return
	}

	// I know, this is horrible, but if you help me write a TLS client for sonic
	// we can asynchronously dial endpoints and remove the need for a goroutine here
	go func() {
		s.handshake(addr, func(err error) {
			s.ioc.Post(func() {
				cb(err)
			})
		})
	}()
}

func (s *WebsocketStream) handshake(addr string, cb func(error)) {
	s.state = StateHandshake

	uri, err := s.resolveAddr(addr)
	if err != nil {
		cb(err)
		s.state = StateTerminated
	} else {
		s.dial(uri, func(err error) {
			if err != nil {
				cb(err)
				s.state = StateTerminated
			} else {
				err = s.upgrade(uri)
				if err != nil {
					s.state = StateTerminated
				} else {
					s.state = StateActive
				}
				cb(err)
			}
		})
	}
}

func (s *WebsocketStream) resolveAddr(addr string) (*url.URL, error) {
	url, err := url.Parse(addr)
	if err != nil {
		return nil, err
	}

	switch url.Scheme {
	case "ws":
		url.Scheme = "http"
	case "wss":
		url.Scheme = "https"
	default:
		return nil, fmt.Errorf("invalid address %s", addr)
	}

	return url, nil
}

func (s *WebsocketStream) dial(uri *url.URL, cb func(err error)) {
	var conn net.Conn
	var sc syscall.Conn
	var err error

	if uri.Scheme == "http" {
		port := uri.Port()
		if port == "" {
			port = "80"
		}

		addr := uri.Hostname() + ":" + port
		conn, err = net.Dial("tcp", addr)
		if err != nil {
			cb(err)
			return
		}
		sc = conn.(syscall.Conn)
	} else {
		if s.tls == nil {
			cb(fmt.Errorf("wss requested with nil tls config"))
			return
		}
		port := uri.Port()
		if port == "" {
			port = "443"
		}

		addr := uri.Hostname() + ":" + port
		conn, err = tls.Dial("tcp", addr, s.tls) // TODO dial timeout
		if err != nil {
			cb(err)
			return
		}
		sc = conn.(*tls.Conn).NetConn().(syscall.Conn)
	}

	sonic.NewAsyncAdapter(s.ioc, sc, conn, func(err error, stream *sonic.AsyncAdapter) {
		if err != nil {
			cb(err)
		} else {
			s.stream = stream
			cb(nil)
		}
	})
}

func (s *WebsocketStream) upgrade(uri *url.URL) error {
	req, err := http.NewRequest("GET", uri.String(), nil)
	if err != nil {
		return err
	}

	sentKey, expectedKey := s.makeKey()
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Connection", "upgrade")
	req.Header.Set("Sec-WebSocket-Key", string(sentKey))
	req.Header.Set("Sec-Websocket-Version", "13")

	err = req.Write(s.stream)
	if err != nil {
		return err
	}

	n, err := s.stream.Read(s.rb)
	if err != nil {
		return err
	}
	s.rb = s.rb[:n]
	rd := bytes.NewReader(s.rb)
	res, err := http.ReadResponse(bufio.NewReader(rd), req)
	if err != nil {
		return err
	}

	rawRes, err := httputil.DumpResponse(res, true)
	if err != nil {
		return err
	}

	resLen := len(rawRes)
	extra := len(s.rb) - resLen
	if extra > 0 {
		s.rb = s.rb[resLen:]
	} else {
		s.rb = s.rb[:0]
	}

	if !IsUpgradeRes(res) {
		return ErrCannotUpgrade
	}

	if key := res.Header.Get("Sec-WebSocket-Accept"); key != expectedKey {
		return ErrCannotUpgrade
	}

	return nil
}

// makeKey generates the key of Sec-WebSocket-Key header as well as the expected
// response present in Sec-WebSocket-Accept header.
func (s *WebsocketStream) makeKey() (req, res string) {
	// request
	b := make([]byte, 16)
	rand.Read(b)
	req = base64.StdEncoding.EncodeToString(b)

	// response
	var resKey []byte
	resKey = append(resKey, []byte(req)...)
	resKey = append(resKey, GUID...)

	s.hasher.Reset()
	s.hasher.Write(resKey)
	res = base64.StdEncoding.EncodeToString(s.hasher.Sum(nil))

	return
}

func (s *WebsocketStream) Read(b []byte) (t MessageType, n int, err error) {
	if s.state != StateTerminated {
		for {
			t, n, err = s.ReadSome(b)
			if s.readFrame.IsFin() {
				return
			}
		}
	} else {
		return TypeNone, 0, io.EOF
	}
}

func (s *WebsocketStream) ReadSome(b []byte) (t MessageType, n int, err error) {
	if s.state != StateTerminated {
		//s.readFrame.Reset()

		//if s.pendingRead() {
		//n, err = s.readPending(b)
		//} else {
		//var nn int64
		//nn, err = s.readFrame.ReadFrom(s.stream)
		//n = int(nn)
		//}

		//return MessageType(s.readFrame.Opcode()), n, err
		panic("sad")
		return
	} else {
		return TypeNone, 0, io.EOF
	}
}

func (s *WebsocketStream) AsyncRead(b []byte, cb AsyncCallback) {
	if s.state != StateTerminated {
		s.asyncRead(b, 0, cb)
	} else {
		cb(io.EOF, 0, TypeNone)
	}
}

func (s *WebsocketStream) asyncRead(b []byte, readBytes int, cb AsyncCallback) {
	s.asyncReadFrame(b[readBytes:], func(err error, n int, t MessageType) {
		readBytes += n
		if err != nil {
			cb(err, readBytes, t)
		} else {
			s.onAsyncRead(b, readBytes, cb)
		}
	})
}

func (s *WebsocketStream) onAsyncRead(b []byte, readBytes int, cb AsyncCallback) {
	if s.readFrame.IsFin() {
		cb(nil, readBytes, MessageType(s.readFrame.Opcode()))
	} else if readBytes >= len(b) {
		cb(ErrPayloadTooBig, readBytes, TypeNone)
	} else {
		// continue reading frames to complete the current message
		s.asyncRead(b, readBytes, cb)
	}
}

func (s *WebsocketStream) AsyncReadSome(b []byte, cb AsyncCallback) {
	if s.state != StateTerminated {
		s.asyncReadFrame(b, cb)
	} else {
		cb(io.EOF, 0, TypeNone)
	}
}

func (s *WebsocketStream) asyncReadFrame(b []byte, cb AsyncCallback) {
	s.AsyncFlush(func(err error) {
		if err != nil {
			cb(err, 0, TypeNone)
		} else {

			handler := s.getReadHandler(b, cb)
			if s.pendingRead() {
				s.tryReadPending(b, handler)
			} else {
				s.asyncReadFrameHeader(b, handler)
			}
		}
	})
}

func (s *WebsocketStream) tryReadPending(b []byte, cb sonic.AsyncCallback) {
	s.readFrame.Reset()

	s.rd.Reset(s.rb)

	n, err := s.readFrame.ReadFrom(s.rd)

	if err == nil {
		s.rb = s.rb[n:]
		b := util.CopyBytes(b, s.readFrame.payload)
		cb(err, len(b))
	} else {
		s.scheduleCompleteShortRead(b, cb)
	}
}

func (s *WebsocketStream) scheduleCompleteShortRead(b []byte, cb sonic.AsyncCallback) {
	existing := len(s.rb)
	s.stream.AsyncRead(s.rb[existing:cap(s.rb)], func(err error, n int) {
		s.rb = s.rb[:existing+n]
		s.tryReadPending(b, cb)
	})
}

// Returns a handler that is invoked when the frame is fully read or an error occurs during reading.
//
// This should never be called before trying to read a frame.
//
// This handler validates the read frame and updates the state machine of the stream.
func (s *WebsocketStream) getReadHandler(b []byte, cb AsyncCallback) sonic.AsyncCallback {
	return func(err error, n int) {
		// Since this handler can be invoked at any time during a read, we forward the error to the
		// caller if supplied. If no error is supplied, that means that a frame has been
		// successfully read and we can proceed to validate and parse it and update the stream's state.
		if err != nil {
			cb(err, n, TypeNone)
			return
		}

		// cannot receive masked frames from the server
		if s.role == RoleClient && s.readFrame.IsMasked() {
			cb(ErrMaskedFrameFromServer, 0, MessageType(s.readFrame.Opcode()))
			return
		}

		// cannot receive unmasked frames from the client
		if s.role == RoleServer && !s.readFrame.IsMasked() {
			cb(ErrUnmaskedFrameFromClient, 0, MessageType(s.readFrame.Opcode()))
			return
		}

		t := MessageType(s.readFrame.Opcode())
		if s.readFrame.IsControl() {
			switch t {
			case TypeClose:
				switch s.state {
				case StateHandshake:
					panic("unreachable")
				case StateActive:
					s.state = StateClosedByPeer

					// this is supplied to the caller
					b = util.CopyBytes(b, s.readFrame.payload)

					// this is queued for writing
					closeFrame := AcquireFrame()
					s.readFrame.CopyTo(closeFrame)
					s.pendingWrite = append(s.pendingWrite, closeFrame)
				case StateClosedByPeer, StateCloseAcked:
					// ignore - connection already closed
				case StateClosedByUs:
					// we received a reply
					s.state = StateCloseAcked
				case StateTerminated:
					panic("unreachable")
				}
			case TypePing:
				if s.state == StateActive {
					// this is supplied to the caller
					b = util.CopyBytes(b, s.readFrame.payload)

					// this is queued for writing
					pongFrame := AcquireFrame()
					pongFrame.SetFin()
					pongFrame.SetPong()
					pongFrame.SetPayload(s.readFrame.payload)

					s.pendingWrite = append(s.pendingWrite, pongFrame)
				}
			case TypePong:
				// this is supplied to the caller
				b = util.CopyBytes(b, s.readFrame.payload)
			default:
				err = ErrUnknownFrameType
			}
		}
		cb(err, n, t)
	}
}

func (s *WebsocketStream) pendingRead() bool {
	return len(s.rb) > 0
}

func (s *WebsocketStream) asyncReadFrameHeader(b []byte, cb sonic.AsyncCallback) {
	s.readFrame.Reset()

	s.stream.AsyncReadAll(s.readFrame.header[:2], func(err error, n int) {
		if err != nil {
			cb(ErrReadingHeader, 0)
		} else {
			if s.readFrame.IsControl() {
				s.asyncReadControlFrame(b, cb)
			} else {
				s.asyncReadDataFrame(b, cb)
			}
		}
	})
}

func (s *WebsocketStream) asyncReadControlFrame(b []byte, cb sonic.AsyncCallback) {
	if !s.readFrame.IsFin() {
		cb(ErrFragmentedControlFrame, 0)
		return
	}

	if s.readFrame.readMore() > 0 {
		cb(ErrControlFrameTooBig, 0)
		return
	}

	s.asyncReadPayload(b, cb)
}

func (s *WebsocketStream) asyncReadDataFrame(b []byte, cb sonic.AsyncCallback) {
	m := s.readFrame.readMore()
	if m > 0 {
		s.asyncReadFrameExtraLength(m, b, cb)
	} else {
		s.asyncTryReadMask(b, cb)
	}
}

func (s *WebsocketStream) asyncTryReadMask(b []byte, cb sonic.AsyncCallback) {
	if s.readFrame.IsMasked() {
		s.asyncReadFrameMask(b, cb)
	} else {
		s.asyncReadPayload(b, cb)
	}
}

func (s *WebsocketStream) asyncReadFrameExtraLength(m int, b []byte, cb sonic.AsyncCallback) {
	s.stream.AsyncReadAll(s.readFrame.header[2:m+2], func(err error, n int) {
		if err != nil {
			cb(ErrReadingExtendedLength, 0)
		} else {
			if s.readFrame.Len() > MaxFramePayloadSize {
				cb(ErrPayloadTooBig, 0)
			} else {
				s.asyncTryReadMask(b, cb)
			}
		}
	})
}

func (s *WebsocketStream) asyncReadFrameMask(b []byte, cb sonic.AsyncCallback) {
	s.stream.AsyncReadAll(s.readFrame.mask[:4], func(err error, n int) {
		if err != nil {
			cb(ErrReadingMask, 0)
		} else {
			s.asyncReadPayload(b, cb)
		}
	})
}

func (s *WebsocketStream) asyncReadPayload(b []byte, cb sonic.AsyncCallback) {
	if payloadLen := int64(s.readFrame.Len()); payloadLen > 0 {
		if remaining := payloadLen - int64(cap(b)); remaining > 0 {
			cb(ErrPayloadTooBig, 0)
		} else {
			s.stream.AsyncReadAll(b[:payloadLen], func(err error, n int) {
				if err != nil {
					cb(err, n)
				} else {
					cb(nil, n)
				}
			})
		}
	} else {
		cb(nil, 0)
	}
}

func (s *WebsocketStream) AsyncWriteFrame(fr *Frame, cb func(err error)) {
	if err := s.prepareFrameWrite(fr); err != nil {
		cb(err)
		return
	}

	if s.state == StateActive {
		switch fr.Opcode() {
		case OpcodeClose:
			s.AsyncClose(CloseNormal, "", cb)
		case OpcodeText, OpcodeBinary, OpcodePing, OpcodePong:
			s.pendingWrite = append(s.pendingWrite, fr)
			s.AsyncFlush(cb)
		default:
			cb(ErrInvalidFrame)
		}
	} else {
		cb(ErrSendAfterClosing)
	}
}

func (s *WebsocketStream) WriteFrame(fr *Frame) error {
	if err := s.prepareFrameWrite(fr); err != nil {
		return err
	}

	if s.state == StateActive {
		switch fr.Opcode() {
		case OpcodeClose:
			return s.Close(CloseNormal, "")
		case OpcodeText, OpcodeBinary, OpcodePing, OpcodePong:
			s.pendingWrite = append(s.pendingWrite, fr)
			return s.Flush()
		default:
			return ErrInvalidFrame
		}
	} else {
		return ErrSendAfterClosing
	}
}

func (s *WebsocketStream) prepareFrameWrite(fr *Frame) error {
	switch s.role {
	case RoleClient:
		if !fr.IsMasked() {
			return ErrUnmaskedFrameFromClient
		}
	case RoleServer:
		if fr.IsMasked() {
			return ErrMaskedFrameFromServer
		}
	}
	return nil
}

func (s *WebsocketStream) AsyncClose(cc CloseCode, reason string, cb func(err error)) {
	s.prepareClose(cc, reason)
	s.AsyncFlush(cb)
}

func (s *WebsocketStream) Close(cc CloseCode, reason string) error {
	s.prepareClose(cc, reason)
	return s.Flush()
}

func (s *WebsocketStream) prepareClose(cc CloseCode, reason string) {
	if s.state == StateActive {
		s.state = StateClosedByUs

		closeFrame := AcquireFrame()
		closeFrame.SetClose()
		closeFrame.SetFin()

		binary.BigEndian.PutUint16(closeFrame.payload, uint16(cc))
		closeFrame.payload = append(closeFrame.payload[2:], []byte(reason)...)

		s.pendingWrite = append(s.pendingWrite, closeFrame)
	}
}
