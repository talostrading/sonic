package sonic

import (
	"github.com/talostrading/sonic/sonicopts"
	"net"
	"time"

	"github.com/talostrading/sonic/internal"
)

var (
	_ Conn = &sonicConn{}
	_ Conn = &netConn{}
)

type AsyncDialCallback func(error, Conn)

// Dial establishes a sonic.Conn to the remote peer. A connection timeout can be passed into opts.
func Dial(
	ioc *IO,
	network string,
	addr string,
	opts ...sonicopts.Option,
) (Conn, error) {
	sock, err := internal.NewSocket(opts...)
	if err != nil {
		return nil, err
	}

	var timeout time.Duration
	if opt := sonicopts.Get(opts, sonicopts.TypeTimeout); opt != nil {
		timeout = opt.Value().(time.Duration)
	}

	err = sock.ConnectTimeout(network, addr, timeout)
	if err != nil {
		return nil, err
	}

	return newSonicConn(ioc, sock, opts...)
}

func AsyncDial(
	ioc *IO,
	network string,
	addr string,
	cb AsyncDialCallback,
	opts ...sonicopts.Option,
) {
	// TODO needs some work in internal/socket_unix.go
	panic("AsyncDial not yet supported")
}

type sonicConn struct {
	FileDescriptor

	sock *internal.Socket
}

func newSonicConn(
	ioc *IO,
	sock *internal.Socket,
	opts ...sonicopts.Option,
) (Conn, error) {
	c := &sonicConn{sock: sock}

	var err error
	c.FileDescriptor, err = NewFileDescriptor(ioc, sock.Fd, opts...)

	return c, err
}

func (c *sonicConn) LocalAddr() net.Addr {
	return c.sock.LocalAddr
}

func (c *sonicConn) RemoteAddr() net.Addr {
	return c.sock.RemoteAddr
}

func (c *sonicConn) SetDeadline(t time.Time) error {
	panic("not supported")
}

func (c *sonicConn) SetReadDeadline(t time.Time) error {
	panic("not supported")
}

func (c *sonicConn) SetWriteDeadline(t time.Time) error {
	panic("not supported")
}

type netConn struct {
	*AsyncAdapter[net.Conn]

	conn net.Conn
}

// AdaptNetConn adapts a net.Conn into a sonic.Conn which allows asynchronous reads and writes.
func AdaptNetConn(ioc *IO, conn net.Conn, opts ...sonicopts.Option) (Conn, error) {
	c := &netConn{conn: conn}

	var err error
	c.AsyncAdapter, err = NewAsyncAdapter(ioc, conn, opts...)

	return c, err
}

func (c *netConn) LocalAddr() net.Addr {
	return c.conn.LocalAddr()
}

func (c *netConn) RemoteAddr() net.Addr {
	return c.conn.RemoteAddr()
}

func (c *netConn) SetDeadline(t time.Time) error {
	return c.conn.SetDeadline(t)
}

func (c *netConn) SetReadDeadline(t time.Time) error {
	return c.conn.SetReadDeadline(t)
}

func (c *netConn) SetWriteDeadline(t time.Time) error {
	return c.conn.SetWriteDeadline(t)
}
