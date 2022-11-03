package sonic

import (
	"fmt"
	"github.com/talostrading/sonic/sonicopts"
	"net"
	"time"
)

var _ ReconnectingConn = &reconnectingConn{}

type reconnectingConn struct {
	ioc     *IO
	network string
	addr    string
	opts    []sonicopts.Option

	conn Conn

	reconnecting bool

	MaxTimeout     time.Duration
	MinTimeout     time.Duration
	TimeoutScaling int
	MaxRetries     int

	reconnectTimer *Timer
	timeout        time.Duration
	retries        int
}

func NewReconnectingConn(ioc *IO, network, addr string, opts ...sonicopts.Option) (ReconnectingConn, error) {
	conn := &reconnectingConn{
		ioc:     ioc,
		network: network,
		addr:    addr,
		opts:    opts,
	}

	var err error
	conn.reconnectTimer, err = NewTimer(ioc)
	if err != nil {
		return nil, err
	}

	return conn, nil
}

func (c *reconnectingConn) Reconnect() (err error) {
	c.conn, err = Dial(c.ioc, c.network, c.addr, c.opts...)
	return
}

func (c *reconnectingConn) scheduleReconnect() error {
	if c.retries >= c.MaxRetries {
		return fmt.Errorf("reconnect failed")
	}

	c.retries++
	err := c.reconnectTimer.ScheduleOnce(c.timeout, func() {
		err := c.Reconnect()
		if err != nil {
			// and call some callback
			c.timeout = c.MinTimeout
		} else {
			c.increaseTimeout()
			c.scheduleReconnect()
		}
	})
	if err != nil {
		return err
	}
}

func (c *reconnectingConn) increaseTimeout() {
	c.timeout *= time.Duration(c.TimeoutScaling)
	if c.timeout > c.MaxTimeout {
		c.timeout = c.MaxTimeout
	}
}

func (c *reconnectingConn) Read(b []byte) (n int, err error) {
	n, err = c.conn.Read(b)
	if err != nil {
		c.scheduleReconnect()
	}
	return
}

func (c *reconnectingConn) Write(p []byte) (n int, err error) {
	//TODO implement me
	panic("implement me")
}

func (c *reconnectingConn) AsyncRead(b []byte, cb AsyncCallback) {
	//TODO implement me
	panic("implement me")
}

func (c *reconnectingConn) AsyncReadAll(b []byte, cb AsyncCallback) {
	//TODO implement me
	panic("implement me")
}

func (c *reconnectingConn) CancelReads() {
	//TODO implement me
	panic("implement me")
}

func (c *reconnectingConn) AsyncWrite(b []byte, cb AsyncCallback) {
	//TODO implement me
	panic("implement me")
}

func (c *reconnectingConn) AsyncWriteAll(b []byte, cb AsyncCallback) {
	//TODO implement me
	panic("implement me")
}

func (c *reconnectingConn) CancelWrites() {
	//TODO implement me
	panic("implement me")
}

func (c *reconnectingConn) Close() error {
	//TODO implement me
	panic("implement me")
}

func (c *reconnectingConn) Closed() bool {
	//TODO implement me
	panic("implement me")
}

func (c *reconnectingConn) RawFd() int { // TODO this should be obtained through multiple NextLayer calls.
	//TODO implement me
	panic("implement me")
}

func (c *reconnectingConn) Opts() []sonicopts.Option {
	//TODO implement me
	panic("implement me")
}

func (c *reconnectingConn) LocalAddr() net.Addr {
	//TODO implement me
	panic("implement me")
}

func (c *reconnectingConn) RemoteAddr() net.Addr {
	//TODO implement me
	panic("implement me")
}

func (c *reconnectingConn) SetDeadline(t time.Time) error {
	//TODO implement me
	panic("implement me")
}

func (c *reconnectingConn) SetReadDeadline(t time.Time) error {
	//TODO implement me
	panic("implement me")
}

func (c *reconnectingConn) SetWriteDeadline(t time.Time) error {
	//TODO implement me
	panic("implement me")
}

func (c *reconnectingConn) NextLayer() Conn {
	return c.conn
}
