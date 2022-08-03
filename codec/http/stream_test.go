package http

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"testing"

	"github.com/talostrading/sonic"
)

func TestClient(t *testing.T) {
	go func() {
		http.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {

		})

		http.ListenAndServe("localhost:8080", nil)
	}()

	ioc := sonic.MustIO()
	defer ioc.Close()

	url, err := url.Parse("http://localhost:8080/")
	if err != nil {
		t.Fatal(err)
	}

	s, err := NewHttpStream(ioc, nil, RoleClient)
	if err != nil {
		t.Fatal(err)
	}

	n := 5 // do 5 requests
	s.AsyncConnect("localhost:8080", func(err error) {
		if err != nil {
			panic(err)
		}

		req := &http.Request{
			Method:     "GET",
			Host:       "localhost:8080",
			URL:        url,
			ProtoMajor: 1,
			ProtoMinor: 1,
		}

		var onAsyncDo func(err error, res *http.Response)
		onAsyncDo = func(err error, res *http.Response) {
			if err != nil {
				panic(err)
			}

			b, err := httputil.DumpResponse(res, true)
			if err != nil {
				panic(err)
			}
			n--
			fmt.Println(string(b))

			if res.StatusCode != http.StatusOK {
				t.Fatalf("invalid status %d", res.StatusCode)
			}

			if n > 0 {
				s.AsyncDo(req, onAsyncDo)
			}
		}

		s.AsyncDo(req, onAsyncDo)
	})

	for {
		if n <= 0 {
			break
		}
		ioc.RunOne()
	}
}
