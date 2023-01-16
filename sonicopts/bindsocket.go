package sonicopts

import "net"

type optionBindSocket struct {
	addr net.Addr
}

// BindSocket covers the case in which the user wants to bind the socket to a specific address
// when Dialing a remote endpoint.
func BindSocket(addr net.Addr) Option {
	return &optionBindSocket{
		addr: addr,
	}
}

func (o *optionBindSocket) Type() OptionType {
	return TypeBindSocket
}

func (o *optionBindSocket) Value() interface{} {
	return o.addr
}
