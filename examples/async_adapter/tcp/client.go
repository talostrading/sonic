package main

import (
	"flag"
	"fmt"
	"net"
	"runtime"
	"syscall"
	"time"

	"github.com/talostrading/sonic"
	"github.com/talostrading/sonic/util"
)

var n = flag.Int("n", 10, "number of connections")

func main() {
	flag.Parse()

	stats := util.NewStats(100_000, nil)

	ioc := sonic.MustIO()

	var conns []*Conn

	for i := 0; i < *n; i++ {
		conn, err := net.Dial("tcp", ":8080")
		if err != nil {
			panic(err)
		}
		sonic.NewAsyncAdapter(ioc, conn.(syscall.Conn), conn, func(err error, ad *sonic.AsyncAdapter) {
			if err != nil {
				fmt.Println("could not make async adapter")
			} else {
				c_ := NewConn(len(conns), conn, ad)
				conns = append(conns, c_)
			}
		})
	}

	for _, c := range conns {
		c.Run()
	}

	for {
		if runtime.NumGoroutine() > 1 {
			// this is for sanity for now
			panic("more than 1 goroutine")
		}

		start := time.Now()
		if err := ioc.RunOne(); err != nil {
			panic(err)
		}

		stats.Add(float64(time.Now().Sub(start).Milliseconds()))
	}
}

type Conn struct {
	buf  []byte
	ad   *sonic.AsyncAdapter
	conn net.Conn
	id   int
}

func NewConn(id int, conn net.Conn, ad *sonic.AsyncAdapter) *Conn {
	c := &Conn{
		buf: make([]byte, 4096),
		ad:  ad,
		id:  id,
	}
	return c
}

func (c *Conn) Run() {
	c.asyncRead()
}

func (c *Conn) asyncRead() {
	c.ad.AsyncRead(c.buf, c.onAsyncRead)
}

func (c *Conn) onAsyncRead(err error, n int) {
	if err != nil {
		panic(err)
	} else {
		c.buf = c.buf[:n]
		c.asyncRead()
	}
}
