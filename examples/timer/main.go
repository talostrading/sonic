package main

import (
	"time"

	"github.com/talostrading/sonic"
)

func main() {
	io, err := sonic.NewIO(-1)
	if err != nil {
		panic(err)
	}

	timer, err := sonic.NewTimer(io)
	if err != nil {
		panic(err)
	}

	timer.Arm(5*time.Second, func() {
		// we must panic to close the process
		panic("timer fired")
	})

	if err := io.Run(); err != nil {
		panic(err)
	}
}
