package sonic

import (
	"fmt"
	"github.com/talostrading/sonic/internal"
	"github.com/talostrading/sonic/sonicerrors"
	"github.com/talostrading/sonic/sonicopts"
	"io"
	"net"
	"sync/atomic"
	"syscall"
)

// SizeofIPMreqSource I would love to do unsafe.SizeOf  but for a struct with 3 4-byte arrays, it returns 8 on my Mac.
// It should return 12 :). So we add 4 bytes instead which is enough for the source IP.
const SizeofIPMreqSource = syscall.SizeofIPMreq + 4

// IPMreqSource adds Sourceaddr to net.IPMreq
type IPMreqSource struct {
	Multiaddr  [4]byte /* in_addr */
	Interface  [4]byte /* in_addr */
	Sourceaddr [4]byte /* in_addr */
}

type MulticastRequestType int

const (
	JoinGroup = iota
	JoinSourceGroup
	LeaveGroup
	LeaveSourceGroup
	BlockSource
	UnblockSource
)

func (r MulticastRequestType) ToIPv4() int {
	switch r {
	case JoinGroup:
		return syscall.IP_ADD_MEMBERSHIP
	case JoinSourceGroup:
		return syscall.IP_ADD_SOURCE_MEMBERSHIP
	case LeaveGroup:
		return syscall.IP_DROP_MEMBERSHIP
	case LeaveSourceGroup:
		return syscall.IP_DROP_SOURCE_MEMBERSHIP
	case BlockSource:
		return syscall.IP_BLOCK_SOURCE
	case UnblockSource:
		return syscall.IP_UNBLOCK_SOURCE
	default:
		panic("invalid request")
	}
}

func (r MulticastRequestType) ToIPv6() int {
	// TODO not sure how IPv6 works, some are not defined
	// I think the filtering is most likely done on layer 2 (ethernet) level

	switch r {
	case JoinGroup:
		return syscall.IPV6_JOIN_GROUP
	case JoinSourceGroup:
		return syscall.IPV6_JOIN_GROUP
	case LeaveGroup:
		return syscall.IPV6_LEAVE_GROUP
	case LeaveSourceGroup:
		return syscall.IPV6_LEAVE_GROUP
	case BlockSource:
		panic("invalid request")
	case UnblockSource:
		panic("invalid request")
	default:
		panic("invalid request")
	}
}

type multicastClient struct {
	ioc     *IO
	iff     *net.Interface
	boundIP net.IP
	opts    []sonicopts.Option

	fd         int
	pd         internal.PollData
	boundAddr  *net.UDPAddr
	closed     uint32
	dispatched int
}

// NewMulticastClient creates a new multicast client bound to the provided interface and IP. For most cases, the boundIP
// should be net.IPv4zero or net.IPv6zero, unless explicit multicast group filtering is needed - in that case, the
// boundIP should be the multicast group IP.
func NewMulticastClient(
	ioc *IO,
	iff *net.Interface,
	boundIP net.IP,
	opts ...sonicopts.Option,
) (MulticastClient, error) {
	c := &multicastClient{
		ioc:     ioc,
		iff:     iff,
		boundIP: boundIP,
		opts:    opts,

		fd: -1,
	}

	// Set by default in internal/.
	opts = sonicopts.DelOption(sonicopts.TypeNonblocking, opts)

	return c, nil
}

func (c *multicastClient) createConn(port int) error {
	var network, addr string
	if c.boundIP.To4() != nil {
		network = "udp4"
	} else {
		network = "udp6"
	}

	addr = fmt.Sprintf("%s:%d", c.boundIP.String(), port)
	fd, boundAddr, err := internal.CreateSocketUDP(network, addr)
	if err != nil {
		return err
	}

	if boundAddr == nil {
		// logic error
		panic("boundAddr should not be nil")
	}

	var opts []sonicopts.Option
	opts = append(opts, c.opts...)
	opts = append(opts, sonicopts.BindSocket(boundAddr))
	// Allow multiple sockets (so clients) to listen to the same multicast group.
	opts = append(opts, sonicopts.ReusePort(true))

	if err := internal.ApplyOpts(fd, opts...); err != nil {
		return err
	}

	c.fd = fd
	c.pd = internal.PollData{Fd: fd}
	c.boundAddr = boundAddr

	return nil
}

func (c *multicastClient) prepareJoin(multicastAddr *net.UDPAddr) error {
	if !multicastAddr.IP.IsMulticast() {
		return fmt.Errorf("cannot join on non-multicast address %s", multicastAddr)
	}

	if c.boundAddr == nil {
		if err := c.createConn(multicastAddr.Port); err != nil {
			return err
		}
	} else if multicastAddr.Port != c.boundAddr.Port {
		return fmt.Errorf("invalid address %s: client must join groups on the same port", multicastAddr)
	}

	return nil
}

// Join a multicast group.
//
// A MulticastClient may join multiple groups as long as they are all bound to the same port and the client is not
// bound to a specific group address (i.e. it is instead bound to 0.0.0.0).
//
// The caller must ensure they do not join an already joined group.
func (c *multicastClient) Join(multicastAddr *net.UDPAddr) error {
	if err := c.prepareJoin(multicastAddr); err != nil {
		return err
	}
	return makeInterfaceRequest(JoinGroup, c.iff, c.fd, multicastAddr, nil)
}

// JoinSource joins a multicast group, filtering out packets not coming from sourceAddr.
//
// The caller must ensure they do not join an already joined source.
func (c *multicastClient) JoinSource(multicastAddr, sourceAddr *net.UDPAddr) error {
	if err := c.prepareJoin(multicastAddr); err != nil {
		return err
	}
	return makeInterfaceRequest(JoinSourceGroup, c.iff, c.fd, multicastAddr, sourceAddr)
}

// Leave a group.
//
// The caller must ensure they do not leave an already left group.
func (c *multicastClient) Leave(multicastAddr *net.UDPAddr) error {
	return makeInterfaceRequest(LeaveGroup, c.iff, c.fd, multicastAddr, nil)
}

// LeaveSource ...
//
// The caller must ensure they do not leave an already left source from the group.
func (c *multicastClient) LeaveSource(multicastAddr, sourceAddr *net.UDPAddr) error {
	return makeInterfaceRequest(LeaveSourceGroup, c.iff, c.fd, multicastAddr, sourceAddr)
}

// BlockSource ...
//
// The caller must ensure they do not block an already blocked source.
func (c *multicastClient) BlockSource(multicastAddr, sourceAddr *net.UDPAddr) error {
	return makeInterfaceRequest(BlockSource, c.iff, c.fd, multicastAddr, sourceAddr)
}

// UnblockSource ...
//
// The caller must ensure they do not unblock an already unblocked source.
func (c *multicastClient) UnblockSource(multicastAddr, sourceAddr *net.UDPAddr) error {
	return makeInterfaceRequest(UnblockSource, c.iff, c.fd, multicastAddr, sourceAddr)
}

func (c *multicastClient) ReadFrom(b []byte) (n int, from net.Addr, err error) {
	var addr syscall.Sockaddr
	n, addr, err = syscall.Recvfrom(c.fd, b, 0)

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
		n = 0 // error contains the information
	}

	return n, internal.FromSockaddrUDP(addr), err
}

func (c *multicastClient) AsyncReadFrom(b []byte, cb AsyncReadCallbackPacket) {
	c.asyncReadFrom(b, false, cb)
}

func (c *multicastClient) AsyncReadAllFrom(b []byte, cb AsyncReadCallbackPacket) {
	c.asyncReadFrom(b, true, cb)
}

func (c *multicastClient) asyncReadFrom(b []byte, readAll bool, cb AsyncReadCallbackPacket) {
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

func (c *multicastClient) asyncReadNow(b []byte, readBytes int, readAll bool, cb AsyncReadCallbackPacket) {
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

func (c *multicastClient) scheduleRead(b []byte, readBytes int, readAll bool, cb AsyncReadCallbackPacket) {
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

func (c *multicastClient) getReadHandler(b []byte, readBytes int, readAll bool, cb AsyncReadCallbackPacket) internal.Handler {
	return func(err error) {
		delete(c.ioc.pendingReads, &c.pd)

		if err != nil {
			cb(err, readBytes, nil)
		} else {
			c.asyncReadNow(b, readBytes, readAll, cb)
		}
	}
}

func (c *multicastClient) setRead() error {
	return c.ioc.poller.SetRead(c.fd, &c.pd)
}

func (c *multicastClient) RawFd() int {
	return c.fd
}

func (c *multicastClient) Interface() *net.Interface {
	return c.iff
}

func (c *multicastClient) LocalAddr() *net.UDPAddr {
	return c.boundAddr
}

// Close closes the client. The caller must make sure to leave all groups with Leave(...) before calling Close.
func (c *multicastClient) Close() error {
	if atomic.CompareAndSwapUint32(&c.closed, 0, 1) {
		return syscall.Close(c.fd)
	}
	return nil
}

func (c *multicastClient) Closed() bool {
	return atomic.LoadUint32(&c.closed) == 1
}
