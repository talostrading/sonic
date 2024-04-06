package main

import (
	"fmt"
	"net"
	"runtime/debug"
	"time"

	"github.com/talostrading/sonic/util"
)

func run(i int, conn net.Conn) {
	stats := util.NewTrackerWithSamples(50000)
	for {
		b := make([]byte, 128)
		start := time.Now()
		n, err := conn.Write(b)
		if err != nil {
			panic(err)
		}
		if n != 128 {
			panic("did not write the entire buffer")
		}

		n, err = conn.Read(b)
		end := time.Now()
		if err != nil {
			panic(err)
		}
		if n != 128 {
			panic("did not read the entire buffer")
		}

		diff := end.Sub(start)
		if result := stats.Record(diff.Microseconds()); result != nil {
			fmt.Printf("%2d min/avg/max/stddev = %f/%f/%f/%fus\n", i, result.Min, result.Avg, result.Max, result.StdDev)
		}
	}
}

func main() {
	debug.SetGCPercent(-1)
	ln, err := net.Listen("tcp", "localhost:1234")
	if err != nil {
		panic(err)
	}
	i := 0
	for {
		i++
		conn, err := ln.Accept()
		fmt.Println("accepted connection", i)
		if err != nil {
			panic(err)
		}
		go run(i, conn)
	}
}
