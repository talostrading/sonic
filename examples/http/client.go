package main

import (
	"bytes"
	"flag"
	"fmt"
	"net/http"
	"net/http/httputil"

	"github.com/talostrading/sonic"
	sonichttp "github.com/talostrading/sonic/codec/http"
)

var (
	nreq = flag.Int("nreq", 1, "the number of requests to do before exiting")
	addr = flag.String("addr", "http://localhost:8080/", "address of http server the client connects to")
)

func main() {
	flag.Parse()

	ioc := sonic.MustIO()
	defer ioc.Close()

	sonichttp.AsyncClient(ioc, *addr, func(err error, client *sonichttp.Client) {
		if err != nil {
			panic(err)
			return
		} else {
			c := NewClient(ioc, client)
			c.Run()
		}
	})

	ioc.Run()
}

type Client struct {
	ioc    *sonic.IO
	client *sonichttp.Client
	nreq   int
}

func NewClient(ioc *sonic.IO, client *sonichttp.Client) *Client {
	return &Client{
		ioc:    ioc,
		client: client,
		nreq:   1,
	}
}

func (c *Client) Run() {
	c.do()
}

func (c *Client) do() {
	if c.nreq <= *nreq {
		fmt.Printf("request #%d\n", c.nreq)
		c.nreq += 1

		req, err := http.NewRequest("GET", "http://localhost:8080/", bytes.NewReader([]byte("hello")))
		if err != nil {
			panic(err)
		} else {
			c.client.Do(req, c.onResponse)
		}
	} else {
		c.ioc.Close()
	}
}

func (c *Client) onResponse(err error, res *http.Response) {
	if err != nil {
		panic(err)
	} else {
		if dump, err := httputil.DumpResponse(res, true); err == nil {
			fmt.Println("--- response ---")
			fmt.Println(string(dump))
			fmt.Println("--- response ---\n")
			c.do()
		} else {
			panic("could not dump response")
		}
	}
}
