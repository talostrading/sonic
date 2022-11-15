package sonicopts

import (
	"crypto/tls"
	"time"
)

func Get(opts []Option, t OptionType) Option {
	for _, opt := range opts {
		if opt.Type() == t {
			return opt
		}
	}
	return nil
}

func Add(opts []Option, toAdd ...Option) []Option {
	for _, existing := range opts {
		for _, opt := range toAdd {
			if existing.Type() == opt.Type() {
				return opts
			}
		}
	}
	opts = append(opts, toAdd...)
	return opts
}

type OptionType uint8

const (
	TypeNonblocking OptionType = iota
	TypeReusePort
	TypeReuseAddr
	TypeNoDelay
	TypeTimeout
	TypeUseNetConn
	TypeTLSConfig
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
	case TypeNoDelay:
		return "no_delay"
	case TypeTimeout:
		return "timeout"
	case TypeUseNetConn:
		return "use_net_conn"
	case TypeTLSConfig:
		return "tls_config"
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

type optionTimeout struct {
	v time.Duration
}

func (o *optionTimeout) Type() OptionType {
	return TypeTimeout
}

func (o *optionTimeout) Value() interface{} {
	return o.v
}

func Timeout(v time.Duration) Option {
	return &optionTimeout{
		v: v,
	}
}

type optionUseNetConn struct {
	v bool
}

func (o *optionUseNetConn) Type() OptionType {
	return TypeUseNetConn
}

func (o *optionUseNetConn) Value() interface{} {
	return o.v
}

func UseNetConn(v bool) Option {
	return &optionUseNetConn{
		v: v,
	}
}

type optionTLSConfig struct {
	v *tls.Config
}

func (o *optionTLSConfig) Type() OptionType {
	return TypeUseNetConn
}

func (o *optionTLSConfig) Value() interface{} {
	return o.v
}

func TLSConfig(v *tls.Config) Option {
	return &optionTLSConfig{
		v: v,
	}
}
