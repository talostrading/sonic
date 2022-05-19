package sonichttp

import (
	"bufio"
	"bytes"
	"context"
	"net"
	"net/http"
	"net/url"
	"syscall"

	"github.com/talostrading/sonic"
)

const BufSize = 4096

type Client struct {
	ioc    *sonic.IO
	ctx    context.Context
	conn   net.Conn
	client *http.Client
	async  *sonic.AsyncAdapter
	wbuf   *bytes.Buffer
	rbuf   []byte
}

func AsyncClient(ioc *sonic.IO, addr string, cb AsyncClientHandler) {
	url, err := url.Parse(addr)
	if err != nil {
		cb(err, nil)
		return
	}

	// TODO AsyncDial (can also fake it in a coroutine for now)
	conn, err := net.Dial("tcp", url.Host)
	if err != nil {
		cb(err, nil)
		return
	}

	c := &Client{
		ioc:  ioc,
		conn: conn,
	}
	c.client = &http.Client{
		Transport: &http.Transport{
			DialContext: c.dial,
		},
	}

	sonic.NewAsyncAdapter(ioc, conn.(syscall.Conn), conn, func(err error, async *sonic.AsyncAdapter) {
		if err != nil {
			cb(err, nil)
		} else {
			c.async = async
			cb(nil, c)
		}
	})
}

func (c *Client) dial(ctx context.Context, network, addr string) (net.Conn, error) {
	c.ctx = ctx
	return c.conn, nil
}

func (c *Client) Do(req *http.Request, cb AsyncResponseHandler) {
	// TODO reuse buffers here
	c.wbuf = bytes.NewBuffer(make([]byte, 0, BufSize))
	c.rbuf = make([]byte, 4096)

	if err := req.Write(c.wbuf); err != nil {
		cb(err, nil)
	} else {
		c.asyncWrite(c.wbuf.Bytes(), req, cb)
	}
}

func (c *Client) asyncWrite(b []byte, req *http.Request, cb AsyncResponseHandler) {
	c.async.AsyncWrite(c.wbuf.Bytes(), func(err error, n int) {
		c.onAsyncWrite(err, n, req, cb)
	})
}

func (c *Client) onAsyncWrite(err error, n int, req *http.Request, cb AsyncResponseHandler) {
	if err != nil {
		cb(err, nil)
	} else {
		c.asyncRead(c.rbuf, req, cb)
	}
}

func (c *Client) asyncRead(b []byte, req *http.Request, cb AsyncResponseHandler) {
	c.async.AsyncRead(c.rbuf, func(err error, n int) {
		c.onAsyncRead(err, n, req, cb)
	})
}

func (c *Client) onAsyncRead(err error, n int, req *http.Request, cb AsyncResponseHandler) {
	if err != nil {
		cb(err, nil)
	} else {
		buf := bytes.NewBuffer(c.rbuf)
		rd := bufio.NewReader(buf)
		if res, err := http.ReadResponse(rd, req); err != nil {
			cb(err, nil)
		} else {
			cb(nil, res)
		}
	}
}
