package sonic

import (
	"fmt"
	"io"
	"net"
	"sync/atomic"
	"syscall"

	"github.com/talostrading/sonic/internal"
	"github.com/talostrading/sonic/sonicerrors"
	"github.com/talostrading/sonic/sonicopts"
)

var _ PacketConn = &packetConn{}

type packetConn struct {
	ioc        *IO
	fd         int
	pd         internal.PollData
	localAddr  net.Addr
	remoteAddr net.Addr
	closed     uint32

	dispatched int
}

// NewPacketConn establishes a packet based stream-less connection which is optionally bound to the specified addr.
//
// If addr is empty, the connection is bound to a random address which can be obtained by calling LocalAddr().
func NewPacketConn(ioc *IO, network, addr string, opts ...sonicopts.Option) (PacketConn, error) {
	if network[:3] != "udp" {
		return nil, fmt.Errorf("network must start with udp for DialPacket")
	}

	fd, localAddr, err := internal.CreateSocketUDP(network, addr)
	if err != nil {
		return nil, err
	}

	if err := syscall.Bind(fd, internal.ToSockaddr(localAddr)); err != nil {
		return nil, err
	}

	return &packetConn{
		ioc:       ioc,
		fd:        fd,
		pd:        internal.PollData{Fd: fd},
		localAddr: localAddr,
		closed:    0,
	}, nil
}

func (c *packetConn) ReadFrom(b []byte) (n int, from net.Addr, err error) {
	var addr syscall.Sockaddr
	n, addr, err = syscall.Recvfrom(c.fd, b, 0)
	from = internal.FromSockaddr(addr)

	if err != nil {
		if err == syscall.EWOULDBLOCK || err == syscall.EAGAIN {
			return 0, nil, sonicerrors.ErrWouldBlock
		}
		return 0, nil, err
	}

	if n == 0 {
		return 0, from, io.EOF
	}

	if n < 0 {
		n = 0 // errors embeds the information
	}

	return n, from, err
}

func (c *packetConn) AsyncReadFrom(b []byte, cb AsyncReadCallbackPacket) {
	c.asyncReadFrom(b, false, cb)
}

func (c *packetConn) AsyncReadAllFrom(b []byte, cb AsyncReadCallbackPacket) {
	c.asyncReadFrom(b, true, cb)
}

func (c *packetConn) asyncReadFrom(b []byte, readAll bool, cb AsyncReadCallbackPacket) {
	if c.dispatched < MaxCallbackDispatch {
		c.asyncReadNow(b, 0, readAll, func(err error, n int, addr net.Addr) {
			c.dispatched++
			cb(err, n, addr)
			c.dispatched--
		})
	} else {
		c.scheduleRead(b, 0, readAll, cb)
	}
}

func (c *packetConn) asyncReadNow(b []byte, readBytes int, readAll bool, cb AsyncReadCallbackPacket) {
	n, addr, err := c.ReadFrom(b)
	readBytes += n

	if err == nil && !(readAll && readBytes != len(b)) {
		cb(err, readBytes, addr)
		return
	}

	if err == sonicerrors.ErrWouldBlock {
		c.scheduleRead(b, readBytes, readAll, cb)
	} else {
		cb(err, readBytes, addr)
	}
}

func (c *packetConn) scheduleRead(b []byte, readBytes int, readAll bool, cb AsyncReadCallbackPacket) {
	if c.Closed() {
		cb(io.EOF, readBytes, nil)
		return
	}

	handler := c.getReadHandler(b, readBytes, readAll, cb)
	c.pd.Set(internal.ReadEvent, handler)

	if err := c.setRead(); err != nil {
		cb(err, readBytes, nil)
	} else {
		c.ioc.pendingReads[&c.pd] = struct{}{}
	}
}

func (c *packetConn) getReadHandler(b []byte, readBytes int, readAll bool, cb AsyncReadCallbackPacket) internal.Handler {
	return func(err error) {
		delete(c.ioc.pendingReads, &c.pd)

		if err != nil {
			cb(err, readBytes, nil)
		} else {
			c.asyncReadNow(b, readBytes, readAll, cb)
		}
	}
}

func (c *packetConn) setRead() error {
	return c.ioc.poller.SetRead(c.fd, &c.pd)
}

func (c *packetConn) WriteTo(b []byte, to net.Addr) error {
	err := syscall.Sendto(c.fd, b, 0, internal.ToSockaddr(to))
	if err == syscall.EWOULDBLOCK || err == syscall.EAGAIN {
		return sonicerrors.ErrWouldBlock
	}
	return err
}

func (c *packetConn) AsyncWriteTo(b []byte, to net.Addr, cb AsyncWriteCallbackPacket) {
	if c.dispatched < MaxCallbackDispatch {
		c.asyncWriteToNow(b, to, func(err error) {
			c.dispatched++
			cb(err)
			c.dispatched--
		})
	} else {
		c.scheduleWrite(b, to, cb)
	}
}

func (c *packetConn) asyncWriteToNow(b []byte, to net.Addr, cb AsyncWriteCallbackPacket) {
	err := c.WriteTo(b, to)
	if err == sonicerrors.ErrWouldBlock {
		c.scheduleWrite(b, to, cb)
	} else {
		cb(err)
	}
}

func (c *packetConn) scheduleWrite(b []byte, to net.Addr, cb AsyncWriteCallbackPacket) {
	if c.Closed() {
		cb(io.EOF)
		return
	}

	handler := c.getWriteHandler(b, to, cb)
	c.pd.Set(internal.WriteEvent, handler)

	if err := c.setWrite(); err != nil {
		cb(err)
	} else {
		c.ioc.pendingWrites[&c.pd] = struct{}{}
	}
}

func (c *packetConn) getWriteHandler(b []byte, to net.Addr, cb AsyncWriteCallbackPacket) internal.Handler {
	return func(err error) {
		delete(c.ioc.pendingWrites, &c.pd)

		if err != nil {
			cb(err)
		} else {
			c.asyncWriteToNow(b, to, cb)
		}
	}
}

func (c *packetConn) setWrite() error {
	return c.ioc.poller.SetWrite(c.fd, &c.pd)
}

func (c *packetConn) Close() error {
	atomic.StoreUint32(&c.closed, 1)
	return syscall.Close(c.fd)
}

func (c *packetConn) Closed() bool {
	return atomic.LoadUint32(&c.closed) == 1
}

func (c *packetConn) LocalAddr() net.Addr {
	return c.localAddr
}

func (c *packetConn) RawFd() int {
	return c.fd
}
