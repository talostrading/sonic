package sonic

import (
	"fmt"
	"github.com/talostrading/sonic/sonicopts"
)

func ListenPacket(
	ioc *IO,
	network, addr string,
	opts ...sonicopts.Option,
) (PacketConn, error) {
	if addr == "" {
		return nil, fmt.Errorf("packet listener needs an address to listen on")
	}
	return NewPacketConn(ioc, network, addr, opts...)
}
