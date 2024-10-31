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

	file.AsyncWrite([]byte("hello, sonic!"), func(err error, n int) {
		if err != nil {
			panic(err)
		}
		fmt.Println("wrote", n, "bytes")

		if _, err := file.Seek(0, 0); err != nil {
			panic(err)
		}

		b := make([]byte, n)
		file.AsyncRead(b, func(err error, n int) {
			fmt.Println("read", n, "bytes:", string(b))
		})
	})

	ioc.RunPending()
}
