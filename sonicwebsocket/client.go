package sonicwebsocket

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"net"

	"github.com/talostrading/sonic"
)

type Client struct {
	ioc        *sonic.IO
	conn       net.Conn
	async      *sonic.AsyncAdapter
	fr         *Frame
	fragmented bool

	// buf holds an entire frame
	buf *bytes.Buffer

	OnControlFrame func(c Opcode)
}

func (c *Client) AsyncWriteText(b []byte, cb sonic.AsyncCallback) {
	fr := AcquireFrame()

	fr.SetFin()
	fr.SetText()
	fr.SetPayload(b)

	fr.Mask()

	c.AsyncWriteFrame(fr, func(err error, n int) {
		cb(err, n)
		ReleaseFrame(fr)
	})
}

func (c *Client) AsyncWriteBinary(b []byte, cb sonic.AsyncCallback) {
	fr := AcquireFrame()

	fr.SetFin()
	fr.SetBinary()
	fr.SetPayload(b)

	fr.Mask()

	c.AsyncWriteFrame(fr, func(err error, n int) {
		cb(err, n)
		ReleaseFrame(fr)
	})
}

func (c *Client) AsyncWriteFrame(fr *Frame, cb sonic.AsyncCallback) {
	nn, err := fr.WriteTo(c.buf)
	n := int(nn)
	if err != nil {
		cb(err, n)
	} else {
		b := c.buf.Bytes()
		b = b[:n]
		c.async.AsyncWriteAll(b, func(err error, n int) {
			cb(err, n)
			c.buf.Reset()
		})
	}
}

func (c *Client) IsFragmented() bool {
	// unfragmented: single frame with FIN bit set and an opcode other than 0 (opcode 0 denotes a continuation frame)
	// fragmented message:
	//	- single frame with FIN bit clear and an opcode other than 0
	//	- followed by zero or more frames with the FIN bit clear and the opcode set to 0
	//	- terminated by a single frame with the FIN bit set and an opcode of 0.
	//	- payload data between fragments is then concatenated
	return c.fragmented
}

func makeRandKey() []byte {
	b := make([]byte, 16)
	rand.Read(b[:])
	n := base64.StdEncoding.EncodedLen(16)
	key := make([]byte, n)
	base64.StdEncoding.Encode(key, b)
	return key
}
