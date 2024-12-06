package websocket

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/talostrading/sonic"
)

const testDur = "WEBSOCKET_INTEGRATION_TEST_DUR"

func TestClientReadWrite(t *testing.T) {
	sd := os.Getenv(testDur)
	if sd == "" {
		sd = "1"
	}
	idur, err := strconv.ParseInt(sd, 10, 32)
	if err != nil {
		t.Fatal(err)
	}
	dur := time.Duration(idur) * time.Second

	s := NewMockServer()

	go func() {
		defer s.Close()
		if err := s.Accept("localhost:0"); err != nil {
			panic(err)
		}

		b := make([]byte, 128)

		var expect uint32 = 1

		for {
			n, err := s.Read(b)
			if err != nil {
				if err != io.EOF {
					panic(err)
				}
				break
			}
			b = b[:n]

			if given := binary.BigEndian.Uint32(b); given != expect {
				panic(fmt.Errorf("wrong: given=%d expected=%d", given, expect))
			}

			expect++
		}
	}()
	time.Sleep(10 * time.Millisecond)

	ioc := sonic.MustIO()
	defer ioc.Close()

	client, err := NewWebsocketStream(ioc, nil, RoleClient)
	if err != nil {
		t.Fatal(err)
	}

	var (
		onHandshake func(error)
		onWrite     func(error)
		b           = make([]byte, 4)
		i           uint32

		prepare = func() {
			i++
			binary.BigEndian.PutUint32(b, i)
		}
	)
	onHandshake = func(err error) {
		if err != nil {
			if err != io.EOF {
				t.Fatal(err)
			}
		} else {
			prepare()
			client.AsyncWrite(b, TypeText, onWrite)
		}
	}
	onWrite = func(err error) {
		if err != nil {
			if err != io.EOF {
				t.Fatal(err)
			}
		} else {
			prepare()
			client.AsyncWrite(b, TypeText, onWrite)
		}
	}

	client.AsyncHandshake(
		fmt.Sprintf("ws://localhost:%d", s.Port()),
		onHandshake,
	)

	start := time.Now()
	for time.Since(start) < dur {
		ioc.PollOne()
	}
}
