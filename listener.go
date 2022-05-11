package sonic

import "github.com/talostrading/sonic/internal"

var _ Listener = &listener{}

type listener struct {
	ioc  *IO
	sock *internal.Socket
}

func Listen(ioc *IO, network, address string) (Listener, error) {
	sock, err := internal.NewSocket()
	if err != nil {
		return nil, err
	}
	if err := sock.Listen(network, address); err != nil {
		return nil, err
	}
	l := &listener{
		ioc:  ioc,
		sock: sock,
	}
	return l, nil
}

func (l *listener) Accept() (Conn, error) {
	ns, err := l.sock.Accept()
	if err != nil {
		return nil, err
	}

	c := &conn{
		file: &file{
			ioc: l.ioc,
			fd:  ns.Fd,
		},
		sock: ns,
	}

	return c, nil
}

func (l *listener) AsyncAccept(AcceptCallback) {

}

func (l *listener) Close() error {
	return nil
}

func (l *listener) Addr() error {
	return nil
}

func ListenPacket(ioc *IO, network, address string) (PacketConn, error) {
	return nil, nil
}
