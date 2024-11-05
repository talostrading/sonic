package main

import (
	"flag"
	"fmt"
	"strconv"

	"github.com/talostrading/sonic"
	"github.com/talostrading/sonic/codec/websocket"
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

	n, err := getCaseCount()
	if err != nil {
		panic(err)
	}

	if *testCase == -1 {
		fmt.Printf("running against all %d cases\n", n)
		for i := 1; i <= n; i++ {
			runTest(i)
			fmt.Printf("ran %d...\n", i)
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

func getCaseCount() (n int, err error) {
	ioc := sonic.MustIO()
	defer ioc.Close()

	s, err := websocket.NewWebsocketStream(ioc, nil, websocket.RoleClient)
	if err != nil {
		return 0, err
	}

	err = s.Handshake(*addr + "/getCaseCount")
	if err != nil {
		return 0, err
	}

	b := make([]byte, 128)
	_, n, err = s.NextMessage(b)
	if err != nil {
		return 0, err
	}
	b = b[:n]

	var nn int64
	nn, err = strconv.ParseInt(string(b), 10, 32)

	return int(nn), err
}

func runTest(i int) {
	ioc := sonic.MustIO()
	defer ioc.Close()

	s, err := websocket.NewWebsocketStream(ioc, nil, websocket.RoleClient)
	if err != nil {
		panic(err)
	}

	done := false
	s.AsyncHandshake(fmt.Sprintf("%s/runCase?case=%d&agent=sonic", *addr, i), func(err error) {
		if err != nil {
			panic(err)
		}

		b := make([]byte, 1024*1024)

		var onAsyncRead websocket.AsyncMessageCallback

		onAsyncRead = func(err error, n int, mt websocket.MessageType) {
			if err != nil {
				s.Flush()
				done = true
			} else {
				b = b[:n]

				switch mt {
				case websocket.TypeText, websocket.TypeBinary:
					s.AsyncWrite(b, mt, func(err error) {
						if err != nil {
							panic(err)
						}

						b = b[:cap(b)]
						s.AsyncNextMessage(b, onAsyncRead)
					})
				case websocket.TypeClose:
					s.AsyncFlush(func(err error) {
						if err != nil {
							panic(err)
						}
						done = true
					})
				default:
					b = b[:cap(b)]
					s.AsyncNextMessage(b, onAsyncRead)
				}
			}
		}

		s.AsyncNextMessage(b, onAsyncRead)
	})

	for {
		ioc.RunOne()
		if done {
			break
		}
	}
}

func updateReports() {
	fmt.Println("updating reports")
	ioc := sonic.MustIO()

	s, err := websocket.NewWebsocketStream(ioc, nil, websocket.RoleClient)
	if err != nil {
		panic("could not update reports")
	}

	s.AsyncHandshake(*addr+"/updateReports?agent=sonic", func(err error) {
		if err != nil {
			panic("could not update reports")
		} else {
			s.AsyncClose(websocket.CloseNormal, "", func(err error) {
				if err != nil {
					panic(err)
				} else {
					ioc.Close()
				}
			})
		}
	})

	ioc.RunOne()
}
