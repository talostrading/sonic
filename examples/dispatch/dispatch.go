package main

import (
	"fmt"

	"github.com/talostrading/sonic"
)

func main() {
	ioc := sonic.MustIO()

	for i := 0; i < 10; i++ {
		// this copy is needed, otherwise we will dispatch 10, the last value of i, each time
		j := i
		ioc.Post(func() {
			fmt.Println("dispatched: ", j)
		})
	}

	ioc.RunPending()
}
