package sonicopts

type OptionType uint8

const (
	TypeNonblocking OptionType = iota
	TypeReusePort
	TypeReuseAddr
	TypeNoDelay
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
	default:
		return "option_unknown"
	}
}

type Option interface {
	Type() OptionType
	Value() interface{}
}

type optionNonblocking struct {
	v bool
}

func (o *optionNonblocking) Type() OptionType {
	return TypeNonblocking
}

func (o *optionNonblocking) Value() interface{} {
	return o.v
}

func Nonblocking(v bool) Option {
	return &optionNonblocking{
		v: v,
	}
}

type optionReusePort struct {
	v bool
}

func (o *optionReusePort) Type() OptionType {
	return TypeReusePort
}

func (o *optionReusePort) Value() interface{} {
	return o.v
}

func ReusePort(v bool) Option {
	return &optionReusePort{
		v: v,
	}
}

type optionReuseAddr struct {
	v bool
}

func (o *optionReuseAddr) Type() OptionType {
	return TypeReuseAddr
}

func (o *optionReuseAddr) Value() interface{} {
	return o.v
}

func ReuseAddr(v bool) Option {
	return &optionReuseAddr{
		v: v,
	}
}

type optionNoDelay struct {
	v bool
}

func (o *optionNoDelay) Type() OptionType {
	return TypeNoDelay
}

func (o *optionNoDelay) Value() interface{} {
	return o.v
}

func NoDelay(v bool) Option {
	return &optionNoDelay{
		v: v,
	}
}
