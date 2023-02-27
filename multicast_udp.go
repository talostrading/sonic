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

func (r MulticastRequestType) String() string {
	switch r {
	case JoinGroup:
		return "join_group"
	case JoinSourceGroup:
		return "join_source_group"
	case LeaveGroup:
		return "leave_group"
	case LeaveSourceGroup:
		return "leave_source_group"
	case BlockSource:
		return "block_source"
	case UnblockSource:
		return "unblock_source"
	default:
		panic("unknown multicast_request_type")
	}
}

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

type udpMulticastClient struct {
	ioc     *IO
	iff     *net.Interface
	boundIP net.IP
	opts    []sonicopts.Option

	fd         int
	pd         internal.PollData
	boundAddr  *net.UDPAddr
	closed     uint32
	dispatched int

	// from is updated on every read
	from *net.UDPAddr
}

var _ UDPMulticastClient = &udpMulticastClient{}

// NewUDPMulticastClient creates a new multicast client bound to the provided interface and IP. For most cases, the boundIP
// should be net.IPv4zero or net.IPv6zero, unless explicit multicast group filtering is needed - in that case, the
// boundIP should be the multicast group IP.
func NewUDPMulticastClient(
	ioc *IO,
	iff *net.Interface,
	boundIP net.IP,
	opts ...sonicopts.Option,
) (UDPMulticastClient, error) {
	c := &udpMulticastClient{
		ioc:     ioc,
		iff:     iff,
		boundIP: boundIP,
		opts:    opts,

		fd: -1,

		from: &net.UDPAddr{},
	}

	return c, nil
}

func (c *udpMulticastClient) make(port int) error {
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
	// Allow multiple sockets to listen to the same multicast group.
	opts = append(opts, sonicopts.ReusePort(true))

	if err := internal.ApplyOpts(fd, opts...); err != nil {
		return err
	}

	c.fd = fd
	c.pd = internal.PollData{Fd: fd}
	c.boundAddr = boundAddr

	return nil
}

func (c *udpMulticastClient) prepareJoin(multicastAddr *net.UDPAddr) error {
	if !multicastAddr.IP.IsMulticast() {
		return fmt.Errorf("cannot join non-multicast address %s", multicastAddr)
	}
	if c.boundAddr == nil {
		return c.make(multicastAddr.Port)
	} else if multicastAddr.Port != c.boundAddr.Port {
		return fmt.Errorf("invalid address %s: client must join groups on the same port", multicastAddr)
	} else {
		return nil
	}
}

// Join a multicast group.
//
// A MulticastClient may join multiple groups as long as they are all bound to the same port and the client is not
// bound to a specific group address (i.e. it is instead bound to 0.0.0.0).
//
// The caller must ensure they do not join an already joined group.
func (c *udpMulticastClient) Join(multicastAddr *net.UDPAddr) error {
	if err := c.prepareJoin(multicastAddr); err != nil {
		return err
	}
	return makeInterfaceRequest(JoinGroup, c.iff, c.fd, multicastAddr.IP, nil)
}

// JoinSource joins a multicast group, filtering out packets not coming from sourceAddr.
//
// The caller must ensure they do not join an already joined source.
func (c *udpMulticastClient) JoinSource(multicastAddr, sourceAddr *net.UDPAddr) error {
	if err := c.prepareJoin(multicastAddr); err != nil {
		return err
	}
	return makeInterfaceRequest(JoinSourceGroup, c.iff, c.fd, multicastAddr.IP, sourceAddr.IP)
}

// Leave a group.
//
// The caller must ensure they do not leave an already left group.
func (c *udpMulticastClient) Leave(multicastAddr *net.UDPAddr) error {
	return makeInterfaceRequest(LeaveGroup, c.iff, c.fd, multicastAddr.IP, nil)

}

// LeaveSource ...
//
// The caller must ensure they do not leave an already left source from the group.
func (c *udpMulticastClient) LeaveSource(multicastAddr, sourceAddr *net.UDPAddr) error {
	return makeInterfaceRequest(LeaveSourceGroup, c.iff, c.fd, multicastAddr.IP, sourceAddr.IP)
}

// BlockSource ...
//
// The caller must ensure they do not block an already blocked source.
func (c *udpMulticastClient) BlockSource(multicastAddr, sourceAddr *net.UDPAddr) error {
	return makeInterfaceRequest(BlockSource, c.iff, c.fd, multicastAddr.IP, sourceAddr.IP)
}

// UnblockSource ...
//
// The caller must ensure they do not unblock an already unblocked source.
func (c *udpMulticastClient) UnblockSource(multicastAddr, sourceAddr *net.UDPAddr) error {
	return makeInterfaceRequest(UnblockSource, c.iff, c.fd, multicastAddr.IP, sourceAddr.IP)
}

func (c *udpMulticastClient) ReadFrom(b []byte) (n int, from net.Addr, err error) {
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

	return n, internal.FromSockaddrUDP(addr, c.from), err
}

func (c *udpMulticastClient) AsyncReadFrom(b []byte, cb AsyncReadCallbackPacket) {
	if c.dispatched < MaxCallbackDispatch {
		c.asyncReadNow(b, func(err error, n int, addr net.Addr) {
			c.dispatched++
			cb(err, n, addr)
			c.dispatched--
		})
	} else {
		c.scheduleRead(b, cb)
	}
}

func (c *udpMulticastClient) asyncReadNow(b []byte, cb AsyncReadCallbackPacket) {
	n, addr, err := c.ReadFrom(b)

	if err == nil {
		cb(err, n, addr)
		return
	}

	if err == sonicerrors.ErrWouldBlock {
		c.scheduleRead(b, cb)
	} else {
		cb(err, 0, addr)
	}
}

func (c *udpMulticastClient) scheduleRead(b []byte, cb AsyncReadCallbackPacket) {
	if c.Closed() {
		cb(io.EOF, 0, nil)
		return
	}

	handler := c.getReadHandler(b, cb)
	c.pd.Set(internal.ReadEvent, handler)

	if err := c.setRead(); err != nil {
		cb(err, 0, nil)
	} else {
		c.ioc.pendingReads[&c.pd] = struct{}{}
	}
}

func (c *udpMulticastClient) getReadHandler(b []byte, cb AsyncReadCallbackPacket) internal.Handler {
	return func(err error) {
		delete(c.ioc.pendingReads, &c.pd)

		if err != nil {
			cb(err, 0, nil)
		} else {
			c.asyncReadNow(b, cb)
		}
	}
}

func (c *udpMulticastClient) setRead() error {
	return c.ioc.poller.SetRead(c.fd, &c.pd)
}

func (c *udpMulticastClient) RawFd() int {
	return c.fd
}

func (c *udpMulticastClient) Interface() *net.Interface {
	return c.iff
}

func (c *udpMulticastClient) LocalAddr() *net.UDPAddr {
	return c.boundAddr
}

func (c *udpMulticastClient) Close() error {
	if atomic.CompareAndSwapUint32(&c.closed, 0, 1) {
		// TODO maybe shutdown instead? on TCP it is cleaner. Also check TCP then.
		return syscall.Close(c.fd)
	}
	return nil
}

func (c *udpMulticastClient) Closed() bool {
	return atomic.LoadUint32(&c.closed) == 1
}
