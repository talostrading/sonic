package main

import (
	"encoding/binary"
	"flag"
	"fmt"
	"math"
	"os"
	"runtime"
	"runtime/debug"
	"time"

	"github.com/talostrading/sonic"
	"github.com/talostrading/sonic/sonicopts"
	"github.com/talostrading/sonic/util"
)

var (
	n       = flag.Int("n", 10, "number of connections")
	cpu     = flag.Int("cpu", 6, "cpu to pin to")
	samples = flag.Int("samples", 1024, "number of samples to record")
	rate    = flag.Int("rate", 0, "rate")

	done = 0
)

type Conn struct {
	ioc      *sonic.IO
	b        []byte
	conn     sonic.Conn
	id       int
	period   int64 // nanos
	lastTime int64 // nanos epoch
	active   bool

	ignore  int
	index   int
	samples []time.Duration
}

func NewConn(ioc *sonic.IO, id int) (c *Conn, err error) {
	c = &Conn{
		ioc:      ioc,
		b:        make([]byte, 1024),
		samples:  make([]time.Duration, *samples, *samples),
		id:       id,
		period:   math.MaxInt64,
		lastTime: 0,
		active:   true,
	}
	c.conn, err = sonic.Dial(ioc, "tcp", "localhost:8080", sonicopts.NoDelay(true))
	if err != nil {
		return nil, err
	}
	c.conn.AsyncReadMulti(c.b, c.OnRead)
	return c, nil
}

func (c *Conn) OnRead(err error, n int) {
	if !c.active {
		return
	}

	if err != nil {
		if c.index >= len(c.samples) {
			c.active = false
			done++
			c.conn.Close()
			c.conn.Cancel()
			fmt.Println("connection finished", c.id, "min_period", time.Duration(c.period)*time.Nanosecond)
			return
		} else {
			panic(err)
		}
	}

	if n != 1024 {
		return
	}

	if c.ignore < 100 {
		c.ignore++
		return
	}

	now := util.GetMonoTimeNanos()
	if c.index < len(c.samples) {
		c.samples[c.index] = time.Duration(now-int64(binary.LittleEndian.Uint64(c.b))) * time.Nanosecond
		c.index++

		if c.lastTime > 0 {
			diff := now - c.lastTime
			if c.period > diff {
				c.period = diff
			}
		}
		c.lastTime = now
	} else {
		c.conn.Cancel()
	}
}

func main() {
	debug.SetGCPercent(-1)

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	flag.Parse()
	if *rate == 0 {
		panic("rate not provided")
	}

	if err := util.PinTo(*cpu); err != nil {
		panic(err)
	} else {
		fmt.Println("pinned to CPU", *cpu)
	}

	fmt.Printf("%d connections, %d samples per connection, rate=%dHz period=%s\n", *n, *samples, *rate, time.Second/time.Duration(*rate))

	ioc := sonic.MustIO()
	defer ioc.Close()

	var conns []*Conn
	for i := 0; i < *n; i++ {
		fmt.Println("connection", i)
		conn, err := NewConn(ioc, i)
		if err != nil {
			panic(err)
		}
		conns = append(conns, conn)
	}

	for {
		ioc.PollOne()

		if done == *n {
			fmt.Println("all connections have finished")
			filename := fmt.Sprintf("sonic_%dHz_%d.txt", *rate, *n)
			file, err := os.OpenFile(filename, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
			if err != nil {
				panic(err)
			}
			defer file.Close()
			for _, conn := range conns {
				for _, sample := range conn.samples {
					file.WriteString(fmt.Sprintf("%d\n", sample.Microseconds()))
				}
			}
			fmt.Println("wrote", filename, ", bye :)")
			return
		}
	}
}
