package sonicwebsocket

type Options uint32

const (
	OptionText Options = 1 << iota
	OptionBinary
)

func (opts Options) validate() {
	if opts.IsSet(OptionText) && opts.IsSet(OptionBinary) {
		panic("cannot have both binary and text set")
	} else if !opts.IsSet(OptionText) && !opts.IsSet(OptionBinary) {
		panic("need to have the binary or text option set")
	}
}

func (opts Options) Set(opt ...Options) {
	for i := range opt {
		opts |= opt[i]
	}
}

func (opts Options) Clear(opt ...Options) {
	for i := range opt {
		opts &= ^opt[i]
	}
}

func (opts Options) IsSet(opt Options) bool {
	if opts&opt == opt {
		return true
	}
	return false
}
