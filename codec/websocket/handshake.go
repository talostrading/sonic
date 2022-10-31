package websocket

import (
	"crypto/sha1"
	"errors"
	"github.com/talostrading/sonic"
	"github.com/talostrading/sonic/codec/http"
	"github.com/talostrading/sonic/sonicerrors"
	"hash"
	"net/url"
)

type Handshake struct {
	// b is the buffer in which we read the handshake response from the peer.
	// After the handshake completes, b might contain other bytes sent by the peer in the read region.
	b *sonic.ByteBuffer

	reqCodec *http.RequestCodec
	resCodec *http.ResponseCodec
	hb       []byte
	hasher   hash.Hash
}

func NewHandshake(b *sonic.ByteBuffer) (*Handshake, error) {
	reqCodec, err := http.NewRequestCodec()
	if err != nil {
		return nil, err
	}

	resCodec, err := http.NewResponseCodec()
	if err != nil {
		return nil, err
	}

	h := &Handshake{
		b: sonic.NewByteBuffer(),

		reqCodec: reqCodec,
		resCodec: resCodec,
		hb:       make([]byte, 16),
		hasher:   sha1.New(),
	}
	return h, nil
}

func (h *Handshake) reset() {
	h.b.Reset()
	h.hasher.Reset()
}

// AsyncDo performs the WebSocket handshake asynchronously, using the provided connection.
//
// This call does not block. If the callback provided error is nil, the handshake completed successfully.
func (h *Handshake) AsyncDo(conn sonic.Conn, url *url.URL, role Role, cb func(error)) {
	switch role {
	case RoleClient:
		h.asyncDoClient(conn, url, cb)
	case RoleServer:
		h.asyncDoServer(conn, url, cb)
	default:
		panic("invalid websocket role")
	}
}

func (h *Handshake) asyncDoClient(conn sonic.Conn, url *url.URL, cb func(error)) {
	h.reset()

	req, expectedKey, err := h.createClientRequest(url)
	if err != nil {
		cb(err)
		return
	}

	if err := h.reqCodec.Encode(req, h.b); err != nil {
		cb(err)
		return
	}

	h.b.CommitAll()

	// No need to check n - the ByteBuffer writes everything.
	h.b.AsyncWriteTo(conn, func(err error, _ int) {
		if err != nil {
			cb(err)
		} else {
			var (
				res    *http.Response
				onRead sonic.AsyncCallback
			)

			onRead = func(err error, _ int) {
				if err != nil {
					cb(err)
				} else {
					res, err = h.resCodec.Decode(h.b)
					if err == nil {
						err = h.checkServerResponse(res, expectedKey)
					}

					if errors.Is(err, sonicerrors.ErrNeedMore) {
						h.b.AsyncReadFrom(conn, onRead)
					} else {
						cb(err)
					}
				}
			}

			h.b.AsyncReadFrom(conn, onRead)
		}

	})
}

func (h *Handshake) asyncDoServer(conn sonic.Conn, url *url.URL, cb func(error)) {
	// TODO
	panic("not yet supported")
}

// Do performs the WebSocket handshake, using the provided connection.
//
// This call blocks until the handshake completes or fails with an error.
func (h *Handshake) Do(conn sonic.Conn, url *url.URL, role Role) error {
	switch role {
	case RoleClient:
		return h.doClient(conn, url)
	case RoleServer:
		return h.doServer(conn, url)
	default:
		panic("invalid websocket role")
	}
}

func (h *Handshake) doClient(conn sonic.Conn, url *url.URL) error {
	h.reset()

	req, expectedKey, err := h.createClientRequest(url)
	if err != nil {
		return err
	}

	if err := h.reqCodec.Encode(req, h.b); err != nil {
		return err
	}

	h.b.CommitAll()

	// No need to check n - the ByteBuffer writes everything.
	if _, err := h.b.WriteTo(conn); err != nil {
		return err
	}

	h.b.Reset()

	var res *http.Response
	for {
		_, err = h.b.ReadFrom(conn)
		if err != nil {
			return err
		}

		res, err = h.resCodec.Decode(h.b)
		if err == nil {
			break
		}

		if !errors.Is(err, sonicerrors.ErrNeedMore) {
			return err
		}
	}

	return h.checkServerResponse(res, expectedKey)
}

func (h *Handshake) doServer(conn sonic.Conn, url *url.URL) error {
	// TODO
	panic("not yet supported")
}

func (h *Handshake) createClientRequest(url *url.URL) (req *http.Request, expectedKey string, err error) {
	var sentKey string

	sentKey = MakeClientRequestKey(h.hb)
	expectedKey = MakeServerResponseKey(h.hasher, []byte(sentKey))

	req, err = http.NewRequest()
	if err != nil {
		return
	}

	req.Method = http.Get
	req.URL = url
	req.Proto = http.ProtoHttp11

	req.Header.Add("Upgrade", "websocket")
	req.Header.Add("Connection", "upgrade")
	req.Header.Add("Sec-WebSocket-Key", sentKey)
	req.Header.Add("Sec-Websocket-Version", "13")

	return
}

func (h *Handshake) checkServerResponse(res *http.Response, expectedKey string) error {
	if !IsUpgradeRes(res) {
		return ErrCannotUpgrade
	}

	if key := res.Header.Get("Sec-WebSocket-Accept"); key != expectedKey {
		return ErrCannotUpgrade
	}

	return nil
}
