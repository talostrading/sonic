package main

import (
	"fmt"

	"github.com/talostrading/sonic"
)

func main() {
	ioc := sonic.MustIO(-1)
	s, err := sonic.Dial(ioc, "tcp", "google.com:80")
	if err != nil {
		panic(err)
	}
	fmt.Println(s)
}
