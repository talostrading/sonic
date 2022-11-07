package sonic

import (
	"fmt"
	"github.com/talostrading/sonic/sonicerrors"
	"github.com/talostrading/sonic/sonicopts"
	"time"
)

var _ ReconnectingConn = &reconnectingConn{}

type reconnectingConn struct {
	Conn

	ioc     *IO
	network string
	addr    string
	opts    []sonicopts.Option

	MaxTimeout     time.Duration
	MinTimeout     time.Duration
	TimeoutScaling int
	MaxRetries     int

	reconnectTimer *Timer
	timeout        time.Duration
	retries        int

	reconnecting bool
	onReconnect  func()
}

func DialReconnecting(ioc *IO, network, addr string, opts ...sonicopts.Option) (ReconnectingConn, error) {
	c := &reconnectingConn{
		ioc:     ioc,
		network: network,
		addr:    addr,
		opts:    opts,

		MaxTimeout:     time.Second,
		MinTimeout:     time.Second,
		timeout:        time.Second,
		TimeoutScaling: 1,
		MaxRetries:     0,
	}

	var err error
	c.reconnectTimer, err = NewTimer(ioc)
	if err != nil {
		return nil, err
	}

	err = c.Reconnect()
	if err != nil {
		return nil, err
	}

	return c, nil
}

func AsyncDialReconnecting(ioc *IO, network, addr string, cb func(ReconnectingConn, error), opts ...sonicopts.Option) {
	// TODO
}

func (c *reconnectingConn) AsyncReconnect(cb func(error)) {
	if c.Conn != nil {
		c.Conn.Close()
	}
	// TODO
}

func (c *reconnectingConn) scheduleAsyncReconnect(cb func(error)) {
	// TODO
}

func (c *reconnectingConn) Reconnect() (err error) {
	if c.Conn != nil {
		c.Conn.Close()
	}
	c.Conn, err = Dial(c.ioc, c.network, c.addr, c.opts...)
	if err == nil {
		c.reconnecting = false
		if c.onReconnect != nil {
			c.onReconnect()
		}
	}
	return
}

func (c *reconnectingConn) scheduleReconnect(actualErr error) error {
	c.reconnecting = true

	if c.MaxRetries > 0 && c.retries >= c.MaxRetries {
		return &sonicerrors.ErrReconnectingFailed{
			Actual:    actualErr,
			Reconnect: fmt.Errorf("retries too many times retries=%d", c.retries),
		}
	} else if c.MaxRetries > 0 {
		c.retries++
	}

	err := c.reconnectTimer.ScheduleOnce(c.timeout, func() {
		err := c.Reconnect()
		if err == nil {
			// and call some callback
			c.timeout = c.MinTimeout
		} else {
			c.increaseTimeout()
			c.scheduleReconnect(actualErr)
		}
	})
	if err != nil {
		return &sonicerrors.ErrReconnectingFailed{
			Actual:    actualErr,
			Reconnect: err,
		}
	}
	return nil
}

func (c *reconnectingConn) increaseTimeout() {
	c.timeout *= time.Duration(c.TimeoutScaling)
	if c.timeout > c.MaxTimeout {
		c.timeout = c.MaxTimeout
	}
}

func (c *reconnectingConn) Read(b []byte) (n int, err error) {
	if c.reconnecting {
		return 0, sonicerrors.ErrReconnecting
	}

	n, err = c.Conn.Read(b)
	if err != nil {
		if wrapper := c.scheduleReconnect(err); wrapper != nil {
			err = wrapper
		}
	}
	return
}

func (c *reconnectingConn) Write(b []byte) (n int, err error) {
	if c.reconnecting {
		return 0, sonicerrors.ErrReconnecting
	}

	n, err = c.Conn.Read(b)
	if err != nil {
		if wrapper := c.scheduleReconnect(err); wrapper != nil {
			err = wrapper
		}
	}
	return
}

func (c *reconnectingConn) AsyncRead(b []byte, cb AsyncCallback) {
	if c.reconnecting {
		cb(sonicerrors.ErrReconnecting, 0)
		return
	}

	c.Conn.AsyncRead(b, func(err error, n int) {
		if err != nil {
			c.scheduleAsyncReconnect(func(err error) {
				cb(err, 0)
			})
		} else {
			cb(err, n)
		}
	})
}

func (c *reconnectingConn) AsyncReadAll(b []byte, cb AsyncCallback) {
	if c.reconnecting {
		cb(sonicerrors.ErrReconnecting, 0)
		return
	}

	c.Conn.AsyncReadAll(b, func(err error, n int) {
		if err != nil {
			c.scheduleAsyncReconnect(func(err error) {
				cb(err, 0)
			})
		} else {
			cb(err, n)
		}
	})
}

func (c *reconnectingConn) AsyncWrite(b []byte, cb AsyncCallback) {
	if c.reconnecting {
		cb(sonicerrors.ErrReconnecting, 0)
		return
	}

	c.Conn.AsyncWrite(b, func(err error, n int) {
		if err != nil {
			c.scheduleAsyncReconnect(func(err error) {
				cb(err, 0)
			})
		} else {
			cb(err, n)
		}
	})
}

func (c *reconnectingConn) AsyncWriteAll(b []byte, cb AsyncCallback) {
	if c.reconnecting {
		cb(sonicerrors.ErrReconnecting, 0)
		return
	}

	c.Conn.AsyncWriteAll(b, func(err error, n int) {
		if err != nil {
			c.scheduleAsyncReconnect(func(err error) {
				cb(err, 0)
			})
		} else {
			cb(err, n)
		}
	})
}

func (c *reconnectingConn) NextLayer() Conn {
	return c.Conn
}

func (c *reconnectingConn) SetOnReconnect(cb func()) {
	c.onReconnect = cb
}
