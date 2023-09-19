package stream

import (
	"github.com/talostrading/sonic"
	"github.com/talostrading/sonic/internal"
)

func Connect(ioc *sonic.IO, transport, addr string) (*Stream, error) {
	fd, localAddr, remoteAddr, err := internal.Connect(transport, addr)
	if err != nil {
		return nil, err
	}
	return newStream(
		ioc,
		fd,
		localAddr,
		remoteAddr,
	)
}
