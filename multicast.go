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
	"unsafe"
)

type multicastClient struct {
	ioc     *IO
	iff     *net.Interface
	boundIP net.IP
	opts    []sonicopts.Option

	fd             int
	pd             internal.PollData
	boundAddr      *net.UDPAddr
	multicastAddrs []*net.UDPAddr
	closed         uint32
	dispatched     int
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

// Join joins the multicast group specified by multicastAddr optionally filtering datagrams by the source addresses in
// sourceAddrs.
//
// A MulticastClient may join multiple groups as long as they are all bound to the same port and the client is not
// bound to a specific group address (i.e. it is instead bound to 0.0.0.0).
func (c *multicastClient) Join(multicastAddr *net.UDPAddr, sourceAddrs ...*net.UDPAddr) error {
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

	c.multicastAddrs = append(c.multicastAddrs, multicastAddr)

	// IPv4 is not compatible with IPv6 which means devices cannot communicate with each other if they mix
	// addressing. So we want an IPv4 interface address for an IPv4 multicast address and same for IPv6.
	if multicastIP := multicastAddr.IP.To4(); multicastIP != nil {
		var (
			errno syscall.Errno
			err   error
		)

		// If we don't dedup IPs then setsockopt will return "can't assign requested address" on a duplicate IP.
		uniqAddrs := make(map[string]*net.UDPAddr)
		for _, addr := range sourceAddrs {
			uniqAddrs[addr.IP.String()] = addr
		}

		if len(sourceAddrs) > 0 {
			for _, sourceAddr := range uniqAddrs {
				mreq, err := createIPv4InterfaceRequestWithSource(multicastIP, c.iff, sourceAddr)
				if err != nil {
					return err
				}

				_, _, errno = syscall.Syscall6(
					syscall.SYS_SETSOCKOPT,
					uintptr(c.fd),
					uintptr(syscall.IPPROTO_IP),
					uintptr(syscall.IP_ADD_SOURCE_MEMBERSHIP),
					uintptr(unsafe.Pointer(mreq)),
					// I would love to do unsafe.SizeOf but for a struct with 3 4-byte arrays, it returns 8 on my Mac.
					// It should return 12 :). So we add 4 bytes instead.
					syscall.SizeofIPMreq+4, 0)
				if errno != 0 {
					err = errno
				}
				if err != nil {
					return err
				}
			}

		} else {
			mreq, err := createIPv4InterfaceRequest(multicastIP, c.iff)
			if err != nil {
				return err
			}

			_, _, errno = syscall.Syscall6(
				syscall.SYS_SETSOCKOPT,
				uintptr(c.fd),
				uintptr(syscall.IPPROTO_IP),
				uintptr(syscall.IP_ADD_MEMBERSHIP),
				uintptr(unsafe.Pointer(mreq)),
				syscall.SizeofIPMreq, 0)
		}

		if errno != 0 {
			err = errno
		}
		return err
	} else {
		mreq, err := createIPv6InterfaceRequest(multicastAddr.IP.To16(), c.iff)
		if err != nil {
			return err
		}

		_, _, errno := syscall.Syscall6(
			syscall.SYS_SETSOCKOPT,
			uintptr(c.fd),
			uintptr(syscall.IPPROTO_IPV6),
			uintptr(syscall.IPV6_JOIN_GROUP),
			uintptr(unsafe.Pointer(mreq)),
			unsafe.Sizeof(mreq), 0,
		)
		if errno != 0 {
			err = errno
		}
		return err
	}
}

func (c *multicastClient) Leave(multicastAddrs ...*net.UDPAddr) error {
	return nil
}

func (c *multicastClient) LeaveAll() error {
	return nil
}

func (c *multicastClient) Block(sourceAddrs ...*net.UDPAddr) error {
	return nil
}

func (c *multicastClient) Unblock(sourceAddrs ...*net.UDPAddr) error {
	return nil
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

func (c *multicastClient) MulticastAddrs() []*net.UDPAddr {
	return c.multicastAddrs
}

func (c *multicastClient) Close() error {
	if atomic.CompareAndSwapUint32(&c.closed, 0, 1) {
		c.LeaveAll()
		return syscall.Close(c.fd)
	}
	return nil
}

func (c *multicastClient) LocalAddr() *net.UDPAddr {
	return c.boundAddr
}

func (c *multicastClient) Closed() bool {
	return atomic.LoadUint32(&c.closed) == 1
}

func serializeIPv4Addr(addr net.Addr, into []byte) bool {
	copyIPv4 := func(ip net.IP) bool {
		if ipv4 := ip.To4(); ipv4 != nil {
			n := copy(into, ipv4)
			if n >= len(ipv4) {
				return true
			}
		}
		return false
	}

	switch addr := addr.(type) {
	case *net.IPAddr:
		return copyIPv4(addr.IP)
	case *net.IPNet:
		return copyIPv4(addr.IP)
	case *net.UDPAddr:
		return copyIPv4(addr.IP)
	}
	return false
}

func createIPv4InterfaceRequest(multicastIP net.IP, iff *net.Interface) (*syscall.IPMreq, error) {
	// set multicast address
	mreq := &syscall.IPMreq{}
	copy(mreq.Multiaddr[:], multicastIP)

	// set interface address
	addrs, err := iff.Addrs()
	if err != nil {
		return nil, err
	}

	for _, addr := range addrs {
		if serializeIPv4Addr(addr, mreq.Interface[:]) {
			return mreq, nil
		}
	}

	return nil, fmt.Errorf(
		"interface name=%s index=%d has not IPv4 address addrs=%v",
		iff.Name, iff.Index, addrs)
}

// IPMreqSource adds Sourceaddr to net.IPMreq
type IPMreqSource struct {
	Multiaddr  [4]byte /* in_addr */
	Interface  [4]byte /* in_addr */
	Sourceaddr [4]byte /* in_addr */
}

func createIPv4InterfaceRequestWithSource(
	multicastIP net.IP,
	iff *net.Interface,
	sourceAddr *net.UDPAddr,
) (*IPMreqSource, error) {
	mreqAll, err := createIPv4InterfaceRequest(multicastIP, iff)
	if err != nil {
		return nil, err
	}

	mreq := &IPMreqSource{}
	copy(mreq.Interface[:], mreqAll.Interface[:])
	copy(mreq.Multiaddr[:], mreqAll.Multiaddr[:])
	if !serializeIPv4Addr(sourceAddr, mreq.Sourceaddr[:]) {
		return nil, fmt.Errorf("source addr %s is not IPv4", sourceAddr)
	}

	return mreq, nil
}

func createIPv6InterfaceRequest(multicastIP net.IP, iff *net.Interface) (*syscall.IPv6Mreq, error) {
	// set multicast address
	mreq := &syscall.IPv6Mreq{}
	copy(mreq.Multiaddr[:], multicastIP)

	// set interface address
	mreq.Interface = uint32(iff.Index)

	return mreq, nil
}
