package sonicwebsocket

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"syscall"

	"github.com/talostrading/sonic"
)

type Client struct {
	ioc   *sonic.IO
	conn  net.Conn
	async *sonic.AsyncAdapter
	fr    *Frame
}

func AsyncDial(ioc *sonic.IO, addr string, cb func(error, *Client)) {
	asyncDial(ioc, addr, nil, cb)
}

func AsyncDialTLS(ioc *sonic.IO, addr string, cnf *tls.Config, cb func(error, *Client)) {
	asyncDial(ioc, addr, cnf, cb)
}

func asyncDial(ioc *sonic.IO, addr string, cnf *tls.Config, cb func(error, *Client)) {
	uri, err := url.Parse(addr)
	if err != nil {
		cb(err, nil)
		return
	}

	var scheme string

	switch uri.Scheme {
	case "ws":
		scheme = "http"
	case "wss":
		scheme = "https"
	default:
		cb(fmt.Errorf("invalid address %s", addr), nil)
		return
	}
	reqURL := strings.Join([]string{scheme, "://", uri.Host, uri.Path}, "")

	// yes this is horrible, but currently the only way to async dial, unless
	// we have a TLS client implementation which can sit on top of sonic.Conn,
	// which can dial asynchronously.
	go func() {
		var err error
		var conn net.Conn
		var sc syscall.Conn

		if scheme == "http" {
			conn, err = net.Dial("tcp", uri.Host)
			sc = conn.(syscall.Conn)
		} else {
			conn, err = tls.Dial("tcp", uri.Host, cnf)
			sc = conn.(*tls.Conn).NetConn().(syscall.Conn)
		}
		if err != nil {
			ioc.Dispatch(func() {
				cb(err, nil)
			})
			return
		}

		cl := &Client{
			ioc:  ioc,
			conn: conn,
			fr:   NewFrame(),
		}

		upgrader := &http.Client{
			Transport: &http.Transport{
				DialContext:     cl.dial,
				DialTLSContext:  cl.dial,
				TLSClientConfig: cnf,
			},
		}

		req, err := http.NewRequest("GET", reqURL, nil)
		if err != nil {
			ioc.Dispatch(func() {
				cb(err, nil)
			})
			return
		}

		req.Header.Set("Upgrade", "websocket")
		req.Header.Set("Connection", "Upgrade")
		req.Header.Set("Sec-WebSocket-Key", string(makeRandKey()))
		req.Header.Set("Sec-Websocket-Version", "13")

		res, err := upgrader.Do(req)
		if err != nil {
			ioc.Dispatch(func() {
				cb(err, nil)
			})
			return
		}

		if res.StatusCode != 101 || res.Header.Get("Upgrade") != "websocket" {
			// TODO check the Sec-Websocket-Accept header as well
			ioc.Dispatch(func() {
				cb(ErrCannotUpgrade, nil)
			})
			return
		}

		sonic.NewAsyncAdapter(ioc, sc, cl.conn, func(err error, async *sonic.AsyncAdapter) {
			if err != nil {
				ioc.Dispatch(func() {
					cb(err, nil)
				})
			} else {
				cl.async = async
				ioc.Dispatch(func() {
					cb(nil, cl)
				})
			}
		})
	}()
}

func (c *Client) dial(ctx context.Context, network, addr string) (net.Conn, error) {
	return c.conn, nil
}

func (c *Client) AsyncRead(b []byte, cb sonic.AsyncCallback) {
	// this mirrors the logic present in frame.ReadFrom(...)
	c.asyncReadHeader(b, cb)
}

func (c *Client) asyncReadHeader(b []byte, cb sonic.AsyncCallback) {
	c.async.AsyncReadAll(c.fr.header[:2], func(err error, n int) {
		if err != nil {
			cb(ErrReadingHeader, -1)
		} else {
			m := c.fr.readMore()
			if m > 0 {
				c.asyncReadLength(m, b, cb)
			} else {
				if c.fr.IsMasked() {
					c.asyncReadMask(b, cb)
				} else {
					c.asyncReadPayload(b, cb)
				}
			}
		}
	})
}

func (c *Client) asyncReadLength(m int, b []byte, cb sonic.AsyncCallback) {
	c.async.AsyncReadAll(c.fr.header[2:m+2], func(err error, n int) {
		if err != nil {
			cb(ErrReadingExtendedLength, -1)
		} else {
			if c.fr.Len() > MaxFramePayloadLen {
				cb(ErrPayloadTooBig, -1)
			} else {
				if c.fr.IsMasked() {
					c.asyncReadMask(b, cb)
				} else {
					c.asyncReadPayload(b, cb)
				}
			}
		}
	})
}

func (c *Client) asyncReadMask(b []byte, cb sonic.AsyncCallback) {
	c.async.AsyncReadAll(c.fr.mask[:4], func(err error, n int) {
		if err != nil {
			cb(ErrReadingMask, -1)
		} else {
			c.asyncReadPayload(b, cb)
		}
	})
}

func (c *Client) asyncReadPayload(b []byte, cb sonic.AsyncCallback) {
	if payloadLen := int(c.fr.Len()); payloadLen > 0 {
		if remaining := payloadLen - int(cap(b)); remaining > 0 {
			cb(ErrPayloadTooBig, -1)
		} else {
			b = b[:payloadLen]
			c.async.AsyncReadAll(b, func(err error, n int) {
				if err != nil {
					cb(err, n)
				} else {
					cb(nil, n)
				}
			})
		}
	} else {
		panic("invalid uint64 to int conversion")
	}
}

func makeRandKey() []byte {
	b := make([]byte, 16)
	rand.Read(b[:])
	n := base64.StdEncoding.EncodedLen(16)
	key := make([]byte, n)
	base64.StdEncoding.Encode(key, b)
	return key
}
