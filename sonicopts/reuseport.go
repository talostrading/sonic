package sonicopts

type optionReusePort struct {
	v bool
}

func ReusePort(v bool) Option {
	return &optionReusePort{
		v: v,
	}
}

func (o *optionReusePort) Type() OptionType {
	return TypeReusePort
}

func (o *optionReusePort) Value() interface{} {
	return o.v
}
