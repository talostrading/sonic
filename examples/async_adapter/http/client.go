package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net"
	"net/http"

	"github.com/talostrading/sonic"
)

func main() {
	ioc := sonic.MustIO()

	AsyncHttpClient(ioc, func(err error, client *HttpClient) {
		if err != nil {
			panic(err)
			return
		}

		req, err := http.NewRequest("GET", "http://localhost:8080", bytes.NewReader([]byte("hello")))
		if err != nil {
			panic(err)
		}

		client.Do(req, func(err error) {
			if err != nil {
				panic(err)
			}
		})
	})

	ioc.RunPending()
}

type HttpClient struct {
	ioc    *sonic.IO
	conn   net.Conn
	client *http.Client
	async  *sonic.AsyncAdapter
	buf    *bytes.Buffer
}

func AsyncHttpClient(ioc *sonic.IO, cb func(error, *HttpClient)) {
	conn, err := net.Dial("tcp", ":8080")
	if err != nil {
		cb(err, nil)
	}

	c := &HttpClient{
		ioc:  ioc,
		conn: conn,
		buf:  bytes.NewBuffer(make([]byte, 0, 4096)),
	}
	c.client = &http.Client{
		Transport: &http.Transport{
			Dial: c.dial,
		},
	}

	sonic.NewAsyncAdapter(ioc, conn, func(err error, async *sonic.AsyncAdapter) {
		if err != nil {
			cb(err, nil)
		} else {
			c.async = async
			cb(nil, c)
		}
	})
}

func (c *HttpClient) dial(network, addr string) (net.Conn, error) {
	return c.conn, nil
}

func (c *HttpClient) Do(req *http.Request, cb func(error)) {
	if err := req.Write(c.buf); err != nil {
		panic("could not buffer request")
	}

	// TODO refactor this is horrible
	// TODO need own buffer implementation
	c.async.AsyncWrite(c.buf.Bytes(), func(err error, n int) {
		if err != nil {
			cb(err)
		} else {
			fmt.Println("wrote", n, "bytes")
			cb(nil)

			buf := make([]byte, 4096)
			c.async.AsyncRead(buf, func(err error, n int) {
				if err != nil {
					panic("could not read response from")
				} else {
					bb := bytes.NewBuffer(buf)
					r := bufio.NewReader(bb)
					res, err := http.ReadResponse(r, req)
					if err != nil {
						panic("could not parse response")
					} else {
						if res.StatusCode != http.StatusOK {
							fmt.Println("not ok http status code", res.StatusCode)
						} else {
							temp, err := io.ReadAll(res.Body)
							if err != nil {
								panic(err)
							}
							fmt.Println("read response from server", string(temp))
						}
					}
				}
			})
		}
	})
}
