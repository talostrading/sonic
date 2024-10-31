package main

import (
	"fmt"
	"os"

	"github.com/csdenboer/sonic"
)

func main() {
	ioc := sonic.MustIO()

	file, err := sonic.Open(ioc, "/tmp/tmp.log", os.O_RDWR|os.O_APPEND|os.O_TRUNC|os.O_CREATE, 0644)
	if err != nil {
		panic(err)
	}
	defer file.Close()

	msg := []byte("hello, sonic!")
	n, err := file.Write(msg)
	if err != nil {
		panic(err)
	}

	fmt.Println("wrote to file: ", n)

	ioc.RunPending()
}
