package main

import (
	"fmt"
	"os"

	"github.com/talostrading/sonic"
)

func main() {
	ioc := sonic.MustIO(-1)

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

		if err := file.Seek(0, sonic.SeekStart); err != nil {
			panic(err)
		}

		b := make([]byte, n+5)
		file.AsyncReadAll(b, func(err error, n int) {
			fmt.Println("read", n, "bytes:", string(b))
		})
	})

	ioc.Run()
}
