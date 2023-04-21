package multicast

import (
	"net/netip"
)

type reactor struct {
	peer *UDPPeer
	b    []byte
	fn   func(error, int, netip.AddrPort)
}

type readReactor struct {
	reactor
}

func (r *readReactor) on(err error) {
	r.peer.ioc.DeregisterRead(&r.peer.slot)

	if err != nil {
		r.fn(err, 0, netip.AddrPort{})
	} else {
		r.peer.asyncReadNow(r.b, r.fn)
	}
}

type writeReactor struct {
	reactor
}

func (r *writeReactor) on(err error) {

}
