package internal

import (
	"os"
	"syscall"
)

type Pipe struct {
	pipe [2]int
	pd   PollData
}

func NewPipe() (*Pipe, error) {
	p := &Pipe{}
	if err := syscall.Pipe(p.pipe[:]); err != nil {
		return nil, err
	}
	p.pd.Fd = p.pipe[0]
	return p, nil
}

func (p *Pipe) SetReadNonblock() error {
	if err := syscall.SetNonblock(p.pipe[0], true); err != nil {
		return os.NewSyscallError("pipe read set_nonblock", err)
	}
	return nil
}

func (p *Pipe) SetWriteNonblock() error {
	if err := syscall.SetNonblock(p.pipe[1], true); err != nil {
		return os.NewSyscallError("pipe write set_nonblock", err)
	}
	return nil
}

func (p *Pipe) Write(b []byte) (n int, err error) {
	n, err = syscall.Write(p.pipe[1], b)
	return
}

func (p *Pipe) Read(b []byte) (n int, err error) {
	n, err = syscall.Read(p.pipe[0], b)
	return
}

func (p *Pipe) ReadFd() int {
	return p.pipe[0]
}

func (p *Pipe) WriteFd() int {
	return p.pipe[1]
}

func (p *Pipe) Close() {
	syscall.Close(p.pipe[0])
	syscall.Close(p.pipe[1])
}
