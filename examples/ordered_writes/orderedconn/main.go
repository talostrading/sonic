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
		conns[i], _ = sonic.QDial(ioc, "tcp", "localhost:8080", queue)
	}

	for i := 0; i < c; i++ {
		conns[i].AsyncWrite([]byte(fmt.Sprintf("Write from %v", i)), func(error, int) {})
	}

	ioc.RunPending()
}
