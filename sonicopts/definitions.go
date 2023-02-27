package sonicopts

import "fmt"

type OptionType uint8

type Option interface {
	Type() OptionType
	Value() interface{}
}

const (
	TypeNonblocking OptionType = iota
	TypeReusePort
	TypeReuseAddr
	TypeNoDelay
	TypeBindSocket
	TypeMulticast
	MaxOption
)

func (t OptionType) String() string {
	switch t {
	case TypeNonblocking:
		return "nonblocking"
	case TypeReusePort:
		return "reuse_port"
	case TypeReuseAddr:
		return "reuse_addr"
	case TypeNoDelay:
		return "no_delay"
	case TypeBindSocket:
		return "bind_socket"
	case TypeMulticast:
		return "multicast"
	default:
		panic(fmt.Errorf("invalid option %d", t))
	}
}

func AddOption(add Option, opts []Option) []Option {
	for i, cur := range opts {
		if cur.Type() == add.Type() {
			opts[i] = add
			return opts
		}
	}
	opts = append(opts, add)
	return opts
}

func DelOption(del OptionType, opts []Option) []Option {
	for i := 0; i < len(opts); i++ {
		if opts[i].Type() == del {
			return append(opts[:i], opts[i+1:]...)
		}
	}
	return opts
}
