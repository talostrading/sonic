package websocket

import (
	"crypto/sha1"
	"errors"
	"fmt"
	"github.com/talostrading/sonic"
	"github.com/talostrading/sonic/codec/http"
	"github.com/talostrading/sonic/sonicerrors"
	"hash"
	"net/url"
)

// TODO expose http headers so the client can request other features

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
		b: b,

		reqCodec: reqCodec,
		resCodec: resCodec,
		hb:       make([]byte, 16),
		hasher:   sha1.New(),
	}
	return h, nil
}

func (h *Handshake) Reset() {
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
	h.Reset()

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
	h.Reset()

	var (
		onReadFromClient sonic.AsyncCallback
		req              *http.Request

		asyncWriteToClient = func(cb func(err error)) {
			res, err := h.createServerResponse(req)
			if err != nil {
				cb(err)
				return
			}

			if err := h.resCodec.Encode(res, h.b); err != nil {
				cb(err)
				return
			}

			h.b.CommitAll()

			h.b.AsyncWriteTo(conn, func(err error, _ int) {
				cb(err)
			})
		}
	)

	onReadFromClient = func(err error, _ int) {
		if err != nil {
			cb(err)
		} else {
			req, err = h.reqCodec.Decode(h.b)
			if err == nil {
				err = h.checkClientRequest(req)
			}

			if errors.Is(err, sonicerrors.ErrNeedMore) {
				h.b.AsyncReadFrom(conn, onReadFromClient)
			} else if err != nil {
				cb(err)
			} else {
				asyncWriteToClient(cb)
			}
		}
	}
	h.b.AsyncReadFrom(conn, onReadFromClient)
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
	h.Reset()

	req, expectedKey, err := h.createClientRequest(url)
	if err != nil {
		return err
	}

	if err := h.reqCodec.Encode(req, h.b); err != nil {
		return err
	}

	h.b.CommitAll()

	// No need to check n as the ByteBuffer writes everything.
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

func (h *Handshake) doServer(conn sonic.Conn, url *url.URL) (err error) {
	h.Reset()

	var req *http.Request
	for {
		_, err = h.b.ReadFrom(conn)
		if err != nil {
			return err
		}

		req, err = h.reqCodec.Decode(h.b)
		if err == nil {
			break
		}

		if !errors.Is(err, sonicerrors.ErrNeedMore) {
			return err
		}
	}

	if err := h.checkClientRequest(req); err != nil {
		return err
	}

	res, err := h.createServerResponse(req)
	if err != nil {
		return err
	}

	if err := h.resCodec.Encode(res, h.b); err != nil {
		return err
	}

	h.b.CommitAll()

	if _, err := h.b.WriteTo(conn); err != nil {
		return err
	}

	return nil
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

	req.Header.Add("Host", url.Host)
	req.Header.Add("Upgrade", "websocket")
	req.Header.Add("Connection", "Upgrade")
	req.Header.Add(HeaderKey, sentKey)
	req.Header.Add(HeaderVersion, DefaultVersion)

	return
}

func (h *Handshake) createServerResponse(req *http.Request) (res *http.Response, err error) {
	res, err = http.NewResponse()
	if err != nil {
		return nil, err
	}

	res.Proto = http.ProtoHttp11
	res.StatusCode = http.StatusSwitchingProtocols
	res.Status = http.StatusText(res.StatusCode)

	res.Header.Add("Upgrade", "websocket")
	res.Header.Add("Connection", "Upgrade")
	res.Header.Add(HeaderAccept, MakeServerResponseKey(h.hasher, []byte(req.Header.Get(HeaderKey))))

	return
}

func (h *Handshake) checkClientRequest(req *http.Request) error {
	if !req.Header.Has(HeaderKey) {
		return fmt.Errorf("handshake failed - client must provide Sec-WebSocket-Key")
	}
	return nil
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
