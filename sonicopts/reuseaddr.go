package sonicopts

type optionReuseAddr struct {
	v bool
}

func ReuseAddr(v bool) Option {
	return &optionReuseAddr{
		v: v,
	}
}

func (o *optionReuseAddr) Type() OptionType {
	return TypeReuseAddr
}

func (o *optionReuseAddr) Value() interface{} {
	return o.v
}
