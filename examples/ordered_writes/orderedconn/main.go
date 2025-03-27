package main

import (
	"fmt"
	"github.com/talostrading/sonic"
)

func main() {

	ioc := sonic.MustIO()
	defer ioc.Close()
	c := 10
	queue := sonic.NewQueue()

	conns := make([]sonic.Conn, c)

	for i := 0; i < c; i++ {
		conns[i], _ = sonic.DialQ(ioc, "tcp", "localhost:8080", queue)
	}

	for i := 0; i < c; i++ {
		j := i
		conns[i].AsyncWrite([]byte(fmt.Sprintf("Write from %v", j)), func(error, int) {})
	}

	ioc.RunPending()
}

func cbFunc(i int, conns []sonic.Conn, j int) func(err error, n int) {
	return func(err error, n int) {
		if err != nil {
			fmt.Printf("could not read from %d err=%v\n", i, err)
		} else {
			conns[j].Close()
		}
	}
}
