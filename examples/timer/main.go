package main

import (
	"fmt"
	"time"

	"github.com/talostrading/sonic"
)

func main() {
	ioc := sonic.MustIO()

	timer, err := sonic.NewTimer(ioc)
	if err != nil {
		panic(err)
	}

	fmt.Println("timer armed: ", time.Now())
	timer.Arm(5*time.Second, func() {
		fmt.Println("timer fired: ", time.Now())
	})

	if err := ioc.RunPending(); err != nil && err != sonic.ErrEOF {
		panic(err)
	}
}
