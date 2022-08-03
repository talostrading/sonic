package main

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"

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

	host := "testnet.binancefuture.com"

	done := false
	s.AsyncConnect(host+":443", func(err error) {
		if err != nil {
			panic(err)
		}

		url, err := url.Parse("http://testnet.binancefuture.com/fapi/v1/time")
		if err != nil {
			panic(err)
		}

		req := &http.Request{
			Method:     "GET",
			URL:        url,
			ProtoMajor: 1,
			ProtoMinor: 1,
			Host:       host,
		}
		s.AsyncDo(req, func(err error, res *http.Response) {
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
