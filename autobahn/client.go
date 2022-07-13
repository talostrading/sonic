package main

import (
	"flag"
	"fmt"
	"strconv"

	"github.com/talostrading/sonic"
	"github.com/talostrading/sonic/sonicwebsocket"
)

var (
	addr     = flag.String("addr", "ws://localhost:9001", "server address")
	testCase = flag.Int("case", -1, "autobahn test case to run")
)

func main() {
	defer func() {
		if err := recover(); err != nil {
			fmt.Println("panicked - updating reports before rethrowing")
			updateReports()
			panic(err)
		}
	}()

	flag.Parse()

	n := getCaseCount()

	if *testCase == -1 {
		fmt.Printf("running against all %d cases\n", n)
		for i := 1; i <= n; i++ {
			runTest(i)
		}
		updateReports()
	} else {
		if *testCase < 1 || *testCase > n {
			panic(fmt.Errorf("invalid test case %d; min=%d max=%d", *testCase, 1, n))
		} else {
			fmt.Printf("running against test case %d\n", *testCase)
			runTest(*testCase)
			updateReports()
		}
	}
}

func getCaseCount() int {
	ioc := sonic.MustIO()
	defer ioc.Close()

	stream, err := sonicwebsocket.NewWebsocketStream(ioc, nil, sonicwebsocket.RoleClient)
	if err != nil {
		panic(err)
	}

	err = stream.Handshake(*addr + "/getCaseCount")
	if err != nil {
		panic(err)
	}
	b := make([]byte, 128)
	t, n, err := stream.Read(b)
	if err != nil {
		panic(err)
	}
	fmt.Println(t, n, err, b)
	b = b[:n]
	nn, err := strconv.ParseInt(string(b), 10, 32)
	if err != nil {
		panic(err)
	}

	return int(nn)
}

func runTest(i int) {
}

func updateReports() {
	fmt.Println("updating reports")
	ioc := sonic.MustIO()

	stream, err := sonicwebsocket.NewWebsocketStream(ioc, nil, sonicwebsocket.RoleClient)
	if err != nil {
		panic("could not update reports")
	}

	stream.AsyncHandshake(*addr+"/updateReports?agent=sonic", func(err error) {
		if err != nil {
			panic("could not update reports")
		} else {
			stream.AsyncClose(sonicwebsocket.CloseNormal, "", func(err error) {
				if err != nil {
					panic(err)
				} else {
					ioc.Close()
				}
			})
		}
	})

	ioc.Run()
}
