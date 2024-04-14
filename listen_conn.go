package sonic

import (
	"net"
	"os"
	"syscall"

	"github.com/talostrading/sonic/internal"
	"github.com/talostrading/sonic/sonicerrors"
	"github.com/talostrading/sonic/sonicopts"
)

var _ Listener = &listener{}

type listener struct {
	ioc  *IO
	slot internal.Slot
	addr net.Addr

	dispatched int
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
	fd, listenAddr, err := internal.Listen(network, addr, opts...)
	if err != nil {
		return nil, err
	}

	l := &listener{
		ioc:  ioc,
		slot: internal.Slot{Fd: fd},
		addr: listenAddr,
	}
	return l, nil
}

func (l *listener) Accept() (Conn, error) {
	return l.accept()
}

func (l *listener) AsyncAccept(cb AcceptCallback) {
	if l.dispatched >= MaxCallbackDispatch {
		l.asyncAccept(cb)
	} else {
		conn, err := l.accept()
		if err != nil && (err == sonicerrors.ErrWouldBlock) {
			l.asyncAccept(cb)
		} else {
			l.dispatched++
			cb(err, conn)
			l.dispatched--
		}
	}
}

func (l *listener) asyncAccept(cb AcceptCallback) {
	l.slot.Set(internal.ReadEvent, l.handleAsyncAccept(cb))

	if err := l.ioc.SetRead(&l.slot); err != nil {
		cb(err, nil)
	} else {
		l.ioc.Register(&l.slot)
	}
}

func (l *listener) handleAsyncAccept(cb AcceptCallback) internal.Handler {
	return func(err error) {
		l.ioc.Deregister(&l.slot)

		if err != nil {
			cb(err, nil)
		} else {
			conn, err := l.accept()
			cb(err, conn)
		}
	}
}

func (l *listener) accept() (Conn, error) {
	fd, addr, err := syscall.Accept(l.slot.Fd)

	if err != nil {
		_ = syscall.Close(fd)
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

	conn := newConn(l.ioc, fd, localAddr, remoteAddr)
	return conn, syscall.SetNonblock(conn.RawFd(), true)
}

func (l *listener) Close() error {
	_ = l.ioc.poller.Del(&l.slot)
	return syscall.Close(l.slot.Fd)
}

func (l *listener) Addr() net.Addr {
	return l.addr
}

func (l *listener) RawFd() int {
	return l.slot.Fd
}
