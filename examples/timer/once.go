package main

import (
	"fmt"
	"io"
	"time"

	"github.com/talostrading/sonic"
)

func main() {
	ioc := sonic.MustIO()

	timer, err := sonic.NewTimer(ioc)
	if err != nil {
		panic(err)
	}
	defer timer.Close()

	fmt.Println("timer armed: ", time.Now())
	err = timer.ScheduleOnce(time.Second, func() {
		fmt.Println("timer fired: ", time.Now())
	})
	if err != nil {
		panic(err)
	}

	if err := ioc.RunPending(); err != nil && err != io.EOF {
		panic(err)
	}
}
