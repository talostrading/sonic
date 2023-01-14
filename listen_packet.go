package sonic

import "github.com/talostrading/sonic/sonicopts"

func ListenPacket(
	ioc *IO,
	network, addr string,
	opts ...sonicopts.Option,
) (PacketConn, error) {
	return NewPacketConn(ioc, network, addr, opts...)
}
