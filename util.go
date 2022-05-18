package sonic

import (
	"fmt"
	"syscall"
)

func GetFd(src interface{}, cb func(err error, fd int)) {
	sc, ok := src.(syscall.Conn)
	if !ok {
		cb(fmt.Errorf("could not get file descriptor"), -1)
	}

	rc, err := sc.SyscallConn()
	if err != nil {
		cb(err, -1)
	}

	rc.Control(func(fd uintptr) {
		cb(nil, int(fd))
	})
}
