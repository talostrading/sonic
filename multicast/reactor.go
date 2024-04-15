package multicast

import (
	"net/netip"
)

type readReactor struct {
	peer *UDPPeer
	b    []byte
	fn   func(error, int, netip.AddrPort)
}

func (r *readReactor) on(err error) {
	r.peer.ioc.Deregister(&r.peer.slot)

	if err != nil {
		r.fn(err, 0, netip.AddrPort{})
	} else {
		r.peer.asyncReadNow(r.b, r.fn)
	}
}

func (r *readReactor) onMulti(err error) {
	if err != nil {
		r.fn(err, 0, netip.AddrPort{})
	} else {
		r.peer.asyncReadMultiNow(r.b, r.fn)
	}
}

type writeReactor struct {
	peer *UDPPeer
	b    []byte
	addr netip.AddrPort
	fn   func(error, int)
}

func (r *writeReactor) on(err error) {
	r.peer.ioc.Deregister(&r.peer.slot)

	if err != nil {
		r.fn(err, 0)
	} else {
		r.peer.asyncWriteNow(r.b, r.addr, r.fn)
	}
}

func (r *writeReactor) onMulti(err error) {
	if err != nil {
		r.fn(err, 0)
	} else {
		r.peer.asyncWriteMultiNow(r.b, r.addr, r.fn)
	}
}

// TODO provide context on why we don't need to guard the stack in here.
