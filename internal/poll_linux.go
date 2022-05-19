//go:build linux

package internal

type PollFlags uint32

const (
	ReadFlags  = PollFlags(syscall.EPOLLIN)
	WriteFlags = PollFlags(syscall.EPOLLOUT)
)

type Event struct {
	Flags uint32
	Data  [8]byte
}

func createEvent(flags PollFlags, pd *PollData) Event {
	ev := Event{
		Flags: uint32(flags),
	}
	*(**PollData)(unsafe.Pointer(&ev.Data)) = pd
}

type Poller struct {
	fd int

	wakeupfd int
	wakeuppd PollData

	lck      sync.Mutex
	handlers []func()

	events []Event

	pending int64

	closed uint32
}

func NewPoller() (*Poller, error) {
	fd, err := unix.EpollCreate1(0)
	if err != nil {
		return nil, err
	}

	efd, _, errno := syscall.Syscall(syscall.SYS_EVENTFD2, 0, syscall.O_NONBLOCK, 0)
	if e0 != 0 {
		syscall.Close(fd)
		return nil, e0
	}

	p := &Poller{
		fd:       fd,
		wakeupfd: int(efd),
		events:   make([]Event, 128),
	}

	err := p.SetRead(p.wakeupfd, &p.wakeuppd)
	if err != nil {
		syscall.Close(p.wakeupfd)
		syscall.Close(p.fd)
	}
	// ignore the waker
	p.pending--

	return p, err
}

func (p *Poller) Pending() int64 {
	return p.pending
}

func (p *Poller) Closed() bool {
	return atomic.LoadUint32(&p.closed) == 1
}

func (p *Poller) Close() error {
	if !atomic.CompareAndSwapUint32(&p.closed, 0, 1) {
		return io.EOF
	}

	p.events = nil
	p.pending = 0

	syscall.Close(p.wakeupfd)
	return syscall.Close(p.fd)
}

func (p *Poller) Dispatch(handler func()) error {
	p.lck.Lock()
	p.handlers = append(p.handlers, handler)
	p.lck.Unlock()

	p.pending++

	// notify the operating system that the event processing loop
	// should run the provided handler
	var x uint64 = 1
	_, err := syscall.Write(p.wakeupfd, (*(*[8]byte)(unsafe.Pointer(&x)))[:])
	return err
}

func (p *Poller) dispatch() {
	for {
		_, err := syscall.Read(p.wakeupfd, p.data[:])
		if err != nil {
			break
		}
	}

	p.lck.Lock()
	for _, handler := range p.handlers {
		// TODO handle pending properly here
		handler()
		p.pending--
	}
	p.lck.Unlock()
}

func (p *Poller) Poll(timeoutMs int) error {
	x, _, errno := syscall.RawSyscall6(
		syscall.SYS_EPOLL_WAIT,
		uintptr(p.fd),
		uintptr(unsafe.Pointer(&p.events[0])),
		uintptr(len(p.events)),
		0, 0,
	)
	if errno != 0 {
		return errno
	}

	if x == 0 && timeoutMs >= 0 {
		return ErrTimeout
	}

	for i := 0; i < int(x); i++ {
		ev := &p.events[i]

		flags := PollFlags(ev.Flags)
		pd := *(**PollData)(unsafe.Pointer(&ev.Data))

		if pd.Fd == p.wakeupfd {
			p.dispatch()
			continue
		}

		if flags&pd.Flags&ReadFlags == ReadFlags {
			p.DelRead(pd.Fd, &pd)
			pd.Cbs[ReadEvent](nil)
		}

		if flags&pd.Flags&WriteFlags == WriteFlags {
			p.DelWrite(pd.Fd, &pd)
			pd.Cbs[WriteEvent](nil)
		}
	}
}

func (p *Poller) DelRead(fd int, pd *PollData) error {
	return nil
}

func (p *Poller) DelWrite(fd int, pd *PollData) error {
	return nil
}

func (p *Poller) SetRead(fd int, pd *PollData) error {
	return nil
}

func (p *Poller) SetWrite(fd int, pd *PollData) error {
	return nil
}
