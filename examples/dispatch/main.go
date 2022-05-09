package main

import (
	"fmt"

	"github.com/talostrading/sonic"
)

func main() {
	ioc := sonic.MustIO(-1)

	for i := 0; i < 10; i++ {
		// this copy is needed, otherwise we will dispatch 10, the last value of i, each time
		j := i
		ioc.Dispatch(func() {
			fmt.Println("dispatched: ", j)
		})
	}

	ioc.RunPending()
}
