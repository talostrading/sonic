package sonicopts

type reuseAddr struct {
	v bool
}

func ReuseAddr(v bool) Option {
	return &reuseAddr{
		v: v,
	}
}

func (o *reuseAddr) Type() OptionType {
	return TypeReuseAddr
}

func (o *reuseAddr) Value() interface{} {
	return o.v
}
