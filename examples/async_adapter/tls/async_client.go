package main

import (
	"crypto/tls"
	"flag"
	"fmt"
	"net"
	"syscall"

	"github.com/talostrading/sonic"
)

var (
	certPath = flag.String("cert", "./certs/client.pem", "client certificate path")
	keyPath  = flag.String("key", "./certs/client.key", "client private key path")
	addr     = flag.String("addr", ":8080", "tls server address")
)

func main() {
	flag.Parse()

	ioc := sonic.MustIO()
	defer ioc.Close()

	NewAsyncClient(ioc, func(err error, c *Client) {
		if err != nil {
			fmt.Println(err)
			panic(err)
		} else {
			c.Run()
		}
	})

	ioc.Run()
}

type Client struct {
	ioc   *sonic.IO
	conn  net.Conn
	async *sonic.AsyncAdapter
	buf   []byte
}

func NewAsyncClient(ioc *sonic.IO, cb func(error, *Client)) {
	cert, err := tls.LoadX509KeyPair(*certPath, *keyPath)
	if err != nil {
		cb(err, nil)
		return
	}

	cfg := tls.Config{
		Certificates:       []tls.Certificate{cert},
		InsecureSkipVerify: true,
	}

	conn, err := tls.Dial("tcp", *addr, &cfg)
	if err != nil {
		cb(err, nil)
		return
	}

	if !conn.ConnectionState().HandshakeComplete {
		cb(fmt.Errorf("handshake not complete"), nil)
		return
	}

	sonic.NewAsyncAdapter(ioc, conn.NetConn().(syscall.Conn), conn, func(err error, async *sonic.AsyncAdapter) {
		if err != nil {
			cb(err, nil)
		} else {
			c := &Client{
				ioc:   ioc,
				conn:  conn,
				async: async,
				buf:   make([]byte, 4096),
			}
			cb(nil, c)
		}
	})
}

func (c *Client) Run() {
	c.asyncWrite()
}

func (c *Client) asyncWrite() {
	c.async.AsyncWrite([]byte("hello"), c.onAsyncWrite)
}

func (c *Client) onAsyncWrite(err error, n int) {
	if err != nil {
		panic(err)
	} else {
		c.asyncRead()
	}
}

func (c *Client) asyncRead() {
	c.async.AsyncRead(c.buf, c.onAsyncRead)
}

func (c *Client) onAsyncRead(err error, n int) {
	if err != nil {
		panic(err)
	} else {
		c.buf = c.buf[:n]
		fmt.Println("client read", string(c.buf))
		fmt.Println("exiting")
		c.ioc.Close()
	}
}
