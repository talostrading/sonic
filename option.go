package sonic

type OptionType uint8

const (
	TypeNonblocking OptionType = iota
)

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
