package sonic

import (
	"fmt"
	"github.com/talostrading/sonic/internal"
	"github.com/talostrading/sonic/sonicopts"
	"net"
	"syscall"
)

// SizeofIPMreqSource I would love to do unsafe.SizeOf  but for a struct with 3 4-byte arrays, it returns 8 on my Mac.
// It should return 12 :). So we add 4 bytes instead which is enough for the source IP.
const SizeofIPMreqSource = syscall.SizeofIPMreq

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
	opts = append(opts, sonicopts.BindBeforeConnect(boundAddr))
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
	return makeInterfaceRequest(JoinGroup, c.iff, c.fd, multicastAddr, nil)
}

// JoinSource joins a multicast group, filtering out packets not coming from sourceAddr.
//
// The caller must ensure they do not join an already joined source.
func (c *udpMulticastClient) JoinSource(multicastAddr, sourceAddr *net.UDPAddr) error {
	if err := c.prepareJoin(multicastAddr); err != nil {
		return err
	}
	return makeInterfaceRequest(JoinSourceGroup, c.iff, c.fd, multicastAddr, sourceAddr)
}

// Leave a group.
//
// The caller must ensure they do not leave an already left group.
func (c *udpMulticastClient) Leave(multicastAddr *net.UDPAddr) error {
	return makeInterfaceRequest(LeaveGroup, c.iff, c.fd, multicastAddr, nil)

}

// LeaveSource ...
//
// The caller must ensure they do not leave an already left source from the group.
func (c *udpMulticastClient) LeaveSource(multicastAddr, sourceAddr *net.UDPAddr) error {
	return makeInterfaceRequest(LeaveSourceGroup, c.iff, c.fd, multicastAddr, sourceAddr)
}

// BlockSource ...
//
// The caller must ensure they do not block an already blocked source.
func (c *udpMulticastClient) BlockSource(multicastAddr *net.UDPAddr) error {
	return makeInterfaceRequest(BlockSource, c.iff, c.fd, multicastAddr, sourceAddr)
}

// UnblockSource ...
//
// The caller must ensure they do not unblock an already unblocked source.
func (c *udpMulticastClient) UnblockSource(multicastAddr, sourceAddr *net.UDPAddr) error {
	return makeInterfaceRequest(UnblockSource, c.iff, c.fd, multicastAddr, sourceAddr)
}

func (c *udpMulticastClient) ReadFrom(bytes []byte) (n int, addr net.Addr, err error) {
	//TODO implement me
	panic("implement me")
}

func (c *udpMulticastClient) AsyncReadFrom(bytes []byte, packet AsyncReadCallbackPacket) {
	//TODO implement me
	panic("implement me")
}

func (c *udpMulticastClient) RawFd() int {
	//TODO implement me
	panic("implement me")
}

func (c *udpMulticastClient) Interface() *net.Interface {
	//TODO implement me
	panic("implement me")
}

func (c *udpMulticastClient) LocalAddr() *net.UDPAddr {
	//TODO implement me
	panic("implement me")
}

func (c *udpMulticastClient) Close() error {
	//TODO implement me
	panic("implement me")
}

func (c *udpMulticastClient) Closed() bool {
	//TODO implement me
	panic("implement me")
}
