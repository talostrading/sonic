package main

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"net/http/httputil"

	"github.com/talostrading/sonic"
	sonichttp "github.com/talostrading/sonic/codec/http"
)

func main() {
	ioc := sonic.MustIO()
	defer ioc.Close()

	s, err := sonichttp.NewHttpStream(ioc, &tls.Config{}, sonichttp.RoleClient)
	if err != nil {
		panic(err)
	}

	done := false
	s.AsyncConnect("https://fapi.binance.com/", func(err error) {
		if err != nil {
			panic(err)
		}

		req := &http.Request{
			Method: "GET",
		}

		s.AsyncDo("/fapi/v1/depth?symbol=BTCUSDT&limit=50", req, func(err error, res *http.Response) {
			if err != nil {
				panic(err)
			}

			b, err := httputil.DumpResponse(res, true)
			if err != nil {
				panic(err)
			}

			fmt.Println(string(b))

			done = true
		})
	})

	for {
		if done {
			break
		}

		ioc.RunOne()
	}
}
