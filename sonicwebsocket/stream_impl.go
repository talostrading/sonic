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
)

var _ Stream = &WebsocketStream{}

// WebsocketStream is a stateful full-duplex connection between two endpoints which
// adheres to the WebSocket protocol.
// The WebsocketStream implements Stream and can be used by both clients and servers.
//
// The underlying socket through which all IO is done is and must remain in blocking mode.
type WebsocketStream struct {
	ioc    *sonic.IO    // executes async operations on behalf of WebsocketStream
	state  StreamState  // current stream state
	tls    *tls.Config  // the optional TLS config used when wanting to connect to `wss` scheme endpoints
	stream sonic.Stream // the stream through which websocket data is sent/received
	role   Role         // role of the stream: client or server

	// closeTimer is a timer which closes the underlying stream on expiry in the client role.
	// The expected behaviour is for the server to close the connection such that the client receives an io.EOF.
	// If that does not happen, this timer does it for the client.
	closeTimer *sonic.Timer

	// Contains the header of the currently read frame.
	rfh *FrameHeader

	// Read buffer which contains the unparsed frames as received from the wire.
	//
	// We try to read as much as possible into the caller's buffer, however there are cases
	// when the peer sends data immediately after the handshake and before the caller
	// schedules any async read. In this instance, the data succeeding the handshake is read into
	// rd and parsed on the next read. Note that rd might contain incomplete frames as a result of short reads.
	rb []byte        // read buffer which contains unparsed frames as received from the wire
	rd *bytes.Reader // reader used to read the above buffer

	pendingWrite []*Frame      // contains frames that are waiting to be written on the wire
	wb           *bytes.Buffer // contains serialized frames ready to be written on the wire

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

		rfh: NewFrameHeader(),

		rb:           make([]byte, MaxPending),
		pendingWrite: make([]*Frame, 0, 128),

		wb: bytes.NewBuffer(make([]byte, DefaultFrameSize)),

		hasher: sha1.New(),
	}
	s.rd = bytes.NewReader(s.rb)

	return s, nil
}

func (s *WebsocketStream) Pending() int {
	return len(s.pendingWrite)
}

func (s *WebsocketStream) Flush() error {
	for i := range s.pendingWrite {
		if _, err := s.write(s.pendingWrite[i]); err != nil {
			s.pendingWrite = s.pendingWrite[i:]
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
		s.asyncWrite(s.pendingWrite[ix], func(err error, n int) {
			ReleaseFrame(s.pendingWrite[ix])
			if err == nil {
				s.asyncFlush(ix+1, cb)
			} else {
				s.pendingWrite = s.pendingWrite[ix:]
				cb(err)
			}
		})
	} else {
		s.pendingWrite = s.pendingWrite[:0]

		cb(nil)
	}
}

func (s *WebsocketStream) asyncWrite(fr *Frame, cb sonic.AsyncCallback) {
	s.wb.Reset()

	_, err := fr.WriteTo(s.wb)
	if err != nil {
		cb(err, 0)
	} else {
		s.stream.AsyncWrite(s.wb.Bytes(), cb)
	}
}

func (s *WebsocketStream) write(fr *Frame) (n int, err error) {
	s.wb.Reset()

	nn, err := fr.WriteTo(s.wb)
	if err != nil {
		return int(nn), err
	} else {
		return s.stream.Write(s.wb.Bytes())
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
			if s.rfh.IsFin() {
				return
			}
		}
	} else {
		return TypeNone, 0, io.EOF
	}
}

func (s *WebsocketStream) ReadSome(b []byte) (t MessageType, n int, err error) {
	if s.state == StateTerminated {
		err = io.EOF
	}

	if err == nil {
		if s.pendingRead() {
			n, err = s.tryReadPending(b)
		} else {
			n, err = s.readFrame(b)
		}
	}

	return s.updateOnRead(b, n, err)
}

func (s *WebsocketStream) tryReadPending(b []byte) (int, error) {
	nt := 0

	for {
		s.rfh.Reset()
		s.rd.Reset(s.rb)

		n, err := s.rfh.Read(b, s.rd)
		nt += n

		if err == nil {
			s.rb = s.rb[nt:]
			return nt, nil
		} else {
			existing := len(s.rb)
			n, err := s.stream.Read(s.rb[existing:cap(s.rb)])
			nt += n

			if err != nil {
				return nt, err
			}

			s.rb = s.rb[:existing+n]
		}
	}
}

func (s *WebsocketStream) readFrame(b []byte) (n int, err error) {
	s.rfh.Reset()
	return s.rfh.Read(b, s.stream)
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
	if s.rfh.IsFin() {
		cb(nil, readBytes, MessageType(s.rfh.Opcode()))
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
	// Before doing a read, we try to flush any pending writes, such as Close/Ping/Pong frames.
	s.AsyncFlush(func(err error) {
		if err != nil {
			cb(err, 0, TypeNone)
		} else {
			handler := s.getReadHandler(b, cb)
			if s.pendingRead() {
				s.asyncTryReadPending(b, handler)
			} else {
				s.asyncReadFrameHeader(b, handler)
			}
		}
	})
}

func (s *WebsocketStream) asyncTryReadPending(b []byte, cb sonic.AsyncCallback) {
	s.rfh.Reset()
	s.rd.Reset(s.rb)

	b = b[:cap(b)]
	n, err := s.rfh.ReadFrom(b, s.rd)
	b = b[:n]

	if err == nil {
		s.rb = s.rb[n:]
		cb(err, len(b))
	} else {
		s.scheduleCompleteShortRead(b, cb)
	}
}

func (s *WebsocketStream) scheduleCompleteShortRead(b []byte, cb sonic.AsyncCallback) {
	existing := len(s.rb)
	s.stream.AsyncRead(s.rb[existing:cap(s.rb)], func(err error, n int) {
		s.rb = s.rb[:existing+n]
		s.asyncTryReadPending(b, cb)
	})
}

func (s *WebsocketStream) pendingRead() bool {
	return len(s.rb) > 0
}

func (s *WebsocketStream) asyncReadFrameHeader(b []byte, cb sonic.AsyncCallback) {
	s.rfh.Reset()

	s.stream.AsyncReadAll(s.rfh.header[:2], func(err error, n int) {
		if err != nil {
			cb(ErrReadingHeader, 0)
		} else {
			if s.rfh.IsControl() {
				s.asyncReadControlFrame(b, cb)
			} else {
				s.asyncReadDataFrame(b, cb)
			}
		}
	})
}

func (s *WebsocketStream) asyncReadControlFrame(b []byte, cb sonic.AsyncCallback) {
	if !s.rfh.IsFin() {
		cb(ErrFragmentedControlFrame, 0)
		return
	}

	if s.rfh.readMore() > 0 {
		cb(ErrControlFrameTooBig, 0)
		return
	}
	fmt.Println("async reading control frame")

	s.asyncReadPayload(b, cb)
}

func (s *WebsocketStream) asyncReadDataFrame(b []byte, cb sonic.AsyncCallback) {
	m := s.rfh.readMore()
	if m > 0 {
		s.asyncReadFrameExtraLength(m, b, cb)
	} else {
		s.asyncTryReadMask(b, cb)
	}
}

func (s *WebsocketStream) asyncTryReadMask(b []byte, cb sonic.AsyncCallback) {
	if s.rfh.IsMasked() {
		s.asyncReadFrameMask(b, cb)
	} else {
		s.asyncReadPayload(b, cb)
	}
}

func (s *WebsocketStream) asyncReadFrameExtraLength(m int, b []byte, cb sonic.AsyncCallback) {
	s.stream.AsyncReadAll(s.rfh.header[2:m+2], func(err error, n int) {
		if err != nil {
			cb(ErrReadingExtendedLength, 0)
		} else {
			if s.rfh.PayloadLen() > MaxFramePayloadSize {
				cb(ErrPayloadTooBig, 0)
			} else {
				s.asyncTryReadMask(b, cb)
			}
		}
	})
}

func (s *WebsocketStream) asyncReadFrameMask(b []byte, cb sonic.AsyncCallback) {
	s.stream.AsyncReadAll(s.rfh.mask[:4], func(err error, n int) {
		if err != nil {
			cb(ErrReadingMask, 0)
		} else {
			s.asyncReadPayload(b, cb)
		}
	})
}

func (s *WebsocketStream) asyncReadPayload(b []byte, cb sonic.AsyncCallback) {
	if pn := s.rfh.PayloadLen(); pn > 0 {
		if remaining := pn - cap(b); remaining > 0 {
			cb(ErrPayloadTooBig, 0)
		} else {
			s.stream.AsyncReadAll(b[:pn], func(err error, n int) {
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

// Returns a handler that will be invoked at some point while trying to complete an async read operation.
// This function is called before trying to perform an async read with the caller's original handler.
// The returned handler wraps the caller's handler in a function that tries to update the state of the stream
// once the async read fails or completes successfully.
func (s *WebsocketStream) getReadHandler(b []byte, cb AsyncCallback) sonic.AsyncCallback {
	return func(err error, n int) {
		t, n, err := s.updateOnRead(b, n, err)
		cb(err, n, t)
	}
}

// Updates the state of the stream after a read operation fails or completes successfully.
// This function should never update anything more than the state of the stream.
// The caller's buffer is not filled by this function but rather during the read operation.
func (s *WebsocketStream) updateOnRead(b []byte, bytesProcessed int, upcallErr error) (t MessageType, n int, err error) {
	// if the read failed, propagate the error to the caller
	if err != nil {
		return TypeNone, 0, upcallErr
	}

	// Number of bytes in the payload. Used by the caller to set the proper buffer length after a successful read.
	n = s.rfh.PayloadLen()

	// at this point, the read succeeded so we proceeed in validating it

	t = MessageType(s.rfh.Opcode())

	// cannot receive masked frames from the server
	if s.role == RoleClient && s.rfh.IsMasked() {
		return t, n, ErrMaskedFrameFromServer
	}

	// cannot receive unmasked frames from the client
	if s.role == RoleServer && !s.rfh.IsMasked() {
		return t, n, ErrUnmaskedFrameFromClient
	}

	// Reading a control frame has consequences for the stream's state and thus warrants an update.
	if s.rfh.IsControl() {
		switch t {
		case TypeClose:
			switch s.state {
			case StateHandshake:
				panic("unreachable")
			case StateActive:
				s.state = StateClosedByPeer

				// this is queued for writing
				closeFrame := AcquireFrame()
				s.rfh.CopyToFrame(closeFrame)

				if s.role == RoleClient {
					closeFrame.Mask(nil)
				}

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
				// this is queued for writing
				pongFrame := AcquireFrame()
				pongFrame.SetFin()
				pongFrame.SetPong()
				pongFrame.SetPayload(b)

				s.pendingWrite = append(s.pendingWrite, pongFrame)
			}
		case TypePong:
		default:
			err = ErrUnknownControlFrameType
		}
	}

	return t, n, err
}

func (s *WebsocketStream) AsyncWriteFrame(fr *Frame, cb func(err error)) {
	if s.state == StateActive {
		s.prepareFrameWrite(fr)
		s.pendingWrite = append(s.pendingWrite, fr)
		s.AsyncFlush(cb)
	} else {
		cb(ErrSendAfterClosing)
	}
}

func (s *WebsocketStream) WriteFrame(fr *Frame) error {
	if s.state == StateActive {
		s.prepareFrameWrite(fr)
		s.pendingWrite = append(s.pendingWrite, fr)
		return s.Flush()
	} else {
		return ErrSendAfterClosing
	}
}

func (s *WebsocketStream) prepareFrameWrite(fr *Frame) {
	switch s.role {
	case RoleClient:
		if !fr.IsMasked() {
			fr.Mask(nil)
		}
	case RoleServer:
		if fr.IsMasked() {
			fr.Unmask(nil)
		}
	}
}

func (s *WebsocketStream) AsyncClose(cc CloseCode, reason string, cb func(err error)) {
	if s.state == StateActive || s.state == StateClosedByPeer {
		s.prepareClose(cc, reason)
		s.AsyncFlush(cb)
	} else {
		cb(ErrOperationAborted)
	}
}

func (s *WebsocketStream) Close(cc CloseCode, reason string) error {
	if s.state == StateActive || s.state == StateClosedByPeer {
		s.prepareClose(cc, reason)
		return s.Flush()
	}
	return ErrOperationAborted
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
