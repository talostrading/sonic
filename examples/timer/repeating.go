package main

import (
	"fmt"
	"io"
	"time"

	"github.com/csdenboer/sonic"
)

func main() {
	ioc := sonic.MustIO()

	timer, err := sonic.NewTimer(ioc)
	if err != nil {
		panic(err)
	}

	fmt.Println("timer armed: ", time.Now())
	i := 0
	err = timer.ScheduleRepeating(time.Second, func() {
		i++
		fmt.Println(i, "timer fired: ", time.Now())
		if i == 5 {
			timer.Close()
		}
	})
	if err != nil {
		panic(err)
	}

	if err := ioc.RunPending(); err != nil && err != io.EOF {
		panic(err)
	}
}
