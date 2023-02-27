package sonicopts

type reusePort struct {
	v bool
}

func ReusePort(v bool) Option {
	return &reusePort{
		v: v,
	}
}

func (o *reusePort) Type() OptionType {
	return TypeReusePort
}

func (o *reusePort) Value() interface{} {
	return o.v
}
