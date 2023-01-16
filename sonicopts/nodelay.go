package sonicopts

type noDelay struct {
	v bool
}

func NoDelay(v bool) Option {
	return &noDelay{
		v: v,
	}
}

func (o *noDelay) Type() OptionType {
	return TypeNoDelay
}

func (o *noDelay) Value() interface{} {
	return o.v
}
