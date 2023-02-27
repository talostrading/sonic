package sonicopts

import "net"

type bindSocket struct {
	addr net.Addr
}

// BindSocket covers the case in which the user wants to bind the socket to a specific address
// when Dialing a remote endpoint.
func BindSocket(addr net.Addr) Option {
	return &bindSocket{
		addr: addr,
	}
}

func (o *bindSocket) Type() OptionType {
	return TypeBindSocket
}

func (o *bindSocket) Value() interface{} {
	return o.addr
}
