package sonicopts

type optionNoDelay struct {
	v bool
}

func NoDelay(v bool) Option {
	return &optionNoDelay{
		v: v,
	}
}

func (o *optionNoDelay) Type() OptionType {
	return TypeNoDelay
}

func (o *optionNoDelay) Value() interface{} {
	return o.v
}
