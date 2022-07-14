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

	n, err := getCaseCount()
	if err != nil {
		panic(err)
	}

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

func getCaseCount() (n int, err error) {
	ioc := sonic.MustIO()
	defer ioc.Close()

	s, err := sonicwebsocket.NewWebsocketStream(ioc, nil, sonicwebsocket.RoleClient)
	if err != nil {
		return 0, err
	}

	err = s.Handshake(*addr + "/getCaseCount")
	if err != nil {
		return 0, err
	}

	b := make([]byte, 128)
	_, n, err = s.Read(b)
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

	s, err := sonicwebsocket.NewWebsocketStream(ioc, nil, sonicwebsocket.RoleClient)
	if err != nil {
		panic(err)
	}

	done := false
	s.AsyncHandshake(fmt.Sprintf("%s/runCase?case=%d&agent=sonic", *addr, i), func(err error) {
		if err != nil {
			panic(err)
		}

		b := make([]byte, 128)
		var onAsyncRead sonicwebsocket.AsyncCallback

		onAsyncRead = func(err error, n int, t sonicwebsocket.MessageType) {
			if err != nil {
				panic(err)
			}

			b = b[:n]

			fmt.Println(err, n, t, string(b), s.Pending())

			switch t {
			case sonicwebsocket.TypeText:
				fr := sonicwebsocket.AcquireFrame()
				fr.SetFin()
				fr.SetPayload(nil)
				fr.SetText()

				s.AsyncWriteFrame(fr, func(err error) {
					sonicwebsocket.ReleaseFrame(fr)

					if err != nil {
						panic(err)
					}

					s.AsyncRead(b, onAsyncRead)
				})
			case sonicwebsocket.TypeClose:
				fmt.Println(s.State(), "READ CLOSE", len(b), n)
				s.AsyncFlush(func(err error) {
					if err != nil {
						panic(err)
					}
				})
				done = true
			default:
				panic(fmt.Errorf("unexpected frame type=%s", t))
			}
		}

		s.AsyncRead(b, onAsyncRead)
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

	s, err := sonicwebsocket.NewWebsocketStream(ioc, nil, sonicwebsocket.RoleClient)
	if err != nil {
		panic("could not update reports")
	}

	s.AsyncHandshake(*addr+"/updateReports?agent=sonic", func(err error) {
		if err != nil {
			panic("could not update reports")
		} else {
			s.AsyncClose(sonicwebsocket.CloseNormal, "", func(err error) {
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
