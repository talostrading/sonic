package main

import (
	"fmt"
	"io"
	"time"

	"github.com/talostrading/sonic"
)

func main() {
	ioc, err := sonic.NewIO(-1)
	if err != nil {
		panic(err)
	}

	timer, err := sonic.NewTimer(ioc)
	if err != nil {
		panic(err)
	}

	timer.Arm(5*time.Second, func() {
		ioc.Close()
	})

	fmt.Println("timer armed: ", time.Now())

	if err := ioc.Run(); err != nil && err != io.EOF {
		panic(err)
	}

	fmt.Println("timer fired: ", time.Now())
}
