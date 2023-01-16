package sonicopts

type nonblocking struct {
	v bool
}

func Nonblocking(v bool) Option {
	return &nonblocking{
		v: v,
	}
}

func (o *nonblocking) Type() OptionType {
	return TypeNonblocking
}

func (o *nonblocking) Value() interface{} {
	return o.v
}
