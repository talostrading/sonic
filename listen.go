package sonic

import (
	"os"
	"syscall"

	"github.com/talostrading/sonic/internal"
	"github.com/talostrading/sonic/sonicerrors"
	"github.com/talostrading/sonic/sonicopts"
)

var _ Listener = &listener{}

type listener struct {
	ioc            *IO
	pd             internal.PollData
	fd             int
	acceptDispatch int
}

// Listen creates a Listener that listens for new connections on the local address.
//
// If the option Nonblocking with value set to false is passed in, you should use Accept()
// to accept incoming connections. In this case, Accept() will block if no connections
// are present in the queue.
//
// If the option Nonblocking with value set to true is passed in, you should use AsyncAccept()
// to accept incoming connections. In this case, AsyncAccept() will not block if no connections
// are present in the queue.
func Listen(
	ioc *IO,
	network,
	addr string,
	opts ...sonicopts.Option,
) (Listener, error) {
	fd, err := internal.Listen(network, addr, opts...)
	if err != nil {
		return nil, err
	}

	l := &listener{
		ioc: ioc,
		pd:  internal.PollData{Fd: fd},
		fd:  fd,
	}
	return l, nil
}

func (l *listener) Accept() (Conn, error) {
	return l.accept()
}

func (l *listener) AsyncAccept(cb AcceptCallback) {
	if l.acceptDispatch >= MaxCallbackDispatch {
		l.asyncAccept(cb)
	} else {
		conn, err := l.accept()
		if err != nil && (err == sonicerrors.ErrWouldBlock) {
			l.asyncAccept(cb)
		} else {
			l.acceptDispatch++
			cb(err, conn)
			l.acceptDispatch--
		}
	}
}

func (l *listener) asyncAccept(cb AcceptCallback) {
	l.pd.Set(internal.ReadEvent, l.handleAsyncAccept(cb))

	if err := l.ioc.poller.SetRead(l.fd, &l.pd); err != nil {
		cb(err, nil)
	} else {
		l.ioc.pendingReads[&l.pd] = struct{}{}
	}
}

func (l *listener) handleAsyncAccept(cb AcceptCallback) internal.Handler {
	return func(err error) {
		delete(l.ioc.pendingReads, &l.pd)

		if err != nil {
			cb(err, nil)
		} else {
			conn, err := l.accept()
			cb(err, conn)
		}
	}
}

func (l *listener) accept() (Conn, error) {
	fd, addr, err := syscall.Accept(l.fd)

	if err != nil {
		syscall.Close(fd)
		if err == syscall.EWOULDBLOCK || err == syscall.EAGAIN {
			return nil, sonicerrors.ErrWouldBlock
		}
		return nil, os.NewSyscallError("accept", err)
	}

	localAddr, err := internal.SocketAddress(fd)
	if err != nil {
		return nil, err
	}

	remoteAddr := internal.FromSockaddr(addr)

	return createConn(l.ioc, fd, localAddr, remoteAddr), nil
}

func (l *listener) Close() error {
	l.ioc.poller.Del(l.fd, &l.pd)
	return syscall.Close(l.fd)
}

func (l *listener) Addr() error {
	// TODO
	return nil
}

func ListenPacket(ioc *IO, network, address string) (PacketConn, error) {
	return nil, nil
}
