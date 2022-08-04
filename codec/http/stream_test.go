package http

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"testing"

	"github.com/talostrading/sonic"
)

func TestClient(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, req *http.Request) {

		}))
	defer srv.Close()

	ioc := sonic.MustIO()
	defer ioc.Close()

	s, err := NewHttpStream(ioc, nil, RoleClient)
	if err != nil {
		t.Fatal(err)
	}

	n := 5 // do 5 requests
	s.AsyncConnect(srv.URL, func(err error) {
		if err != nil {
			t.Fatal(err)
		}

		req := &http.Request{
			Method: "GET",
		}

		var onAsyncDo AsyncResponseHandler
		onAsyncDo = func(err error, res *http.Response) {
			if err != nil {
				t.Fatal(err)
			}

			b, err := httputil.DumpResponse(res, true)
			if err != nil {
				t.Fatal(err)
			}
			fmt.Println(string(b))

			n--

			if res.StatusCode != http.StatusOK {
				t.Fatalf("invalid status %d", res.StatusCode)
			}

			if n > 0 {
				s.AsyncDo("/", req, onAsyncDo)
			}
		}

		s.AsyncDo("/", req, onAsyncDo)
	})

	for {
		if n <= 0 {
			break
		}
		ioc.RunOne()
	}
}

func TestClientCanRetry(t *testing.T) {
	// Test whether we retry the request after the first one fails.
	// The retry should succeed.
	MaxRetries = 1

	respond := false
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if !respond {
			srv.CloseClientConnections()
			respond = true
		} else {
		}
	}))
	defer srv.Close()

	ioc := sonic.MustIO()
	defer ioc.Close()

	s, err := NewHttpStream(ioc, nil, RoleClient)
	if err != nil {
		t.Fatal(err)
	}

	req := &http.Request{
		Method: "GET",
	}

	done := false
	s.AsyncConnect(srv.URL, func(err error) {
		if err != nil {
			t.Fatal(err)
		}

		s.AsyncDo("/", req, func(err error, res *http.Response) {
			if err != nil {
				t.Fatal(err)
			}

			b, err := httputil.DumpResponse(res, true)
			if err != nil {
				t.Fatal(err)
			}

			fmt.Println(string(b))

			if res.StatusCode != http.StatusOK {
				t.Fatal(res.StatusCode)
			}

			if s.State() != StateConnected {
				t.Fatal("expected StateConnected")
			}

			if s.retries != 0 {
				t.Fatal("retries should be 0")
			}

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

func TestClientCannotRetry(t *testing.T) {
	// Test whether we retry the request after the first one fails.
	// The retry should succeed.
	MaxRetries = 0

	respond := false
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if !respond {
			srv.CloseClientConnections()
			respond = true
		} else {
		}
	}))
	defer srv.Close()

	ioc := sonic.MustIO()
	defer ioc.Close()

	s, err := NewHttpStream(ioc, nil, RoleClient)
	if err != nil {
		t.Fatal(err)
	}

	req := &http.Request{
		Method: "GET",
	}

	done := false
	s.AsyncConnect(srv.URL, func(err error) {
		if err != nil {
			t.Fatal(err)
		}

		s.AsyncDo("/", req, func(err error, res *http.Response) {
			if err != io.EOF {
				t.Fatal("expected EOF")
			}

			if s.State() != StateDisconnected {
				t.Fatal("expected StateDisconnected")
			}

			if s.retries != 0 {
				t.Fatal("retries should be 0")
			}

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

func TestClientHandlesCloseResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Connection", "close")
	}))
	defer srv.Close()

	ioc := sonic.MustIO()
	defer ioc.Close()

	s, err := NewHttpStream(ioc, nil, RoleClient)
	if err != nil {
		t.Fatal(err)
	}

	req := &http.Request{
		Method: "GET",
	}

	done := false
	s.AsyncConnect(srv.URL, func(err error) {
		if err != nil {
			t.Fatal(err)
		}

		s.AsyncDo("/", req, func(err error, res *http.Response) {
			b, err := httputil.DumpResponse(res, true)
			if err != nil {
				t.Fatal(err)
			}

			fmt.Println(string(b))

			if s.State() != StateDisconnected {
				t.Fatal("expected StateDisconnected")
			}

			if s.retries != 0 {
				t.Fatal("retries should be 0")
			}

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

func TestClientClosesAfterSendingCloseRequest(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Header.Get("Connection") != "close" {
			t.Fatal("expected Connection: close header")
		}
	}))
	defer srv.Close()

	ioc := sonic.MustIO()
	defer ioc.Close()

	s, err := NewHttpStream(ioc, nil, RoleClient)
	if err != nil {
		t.Fatal(err)
	}

	req := &http.Request{
		Method: "GET",
		Close:  true,
	}

	done := false
	s.AsyncConnect(srv.URL, func(err error) {
		if err != nil {
			t.Fatal(err)
		}

		s.AsyncDo("/", req, func(err error, res *http.Response) {
			b, err := httputil.DumpResponse(res, true)
			if err != nil {
				t.Fatal(err)
			}

			fmt.Println(string(b))

			if s.State() != StateDisconnected {
				t.Fatal("expected StateDisconnected")
			}

			if s.retries != 0 {
				t.Fatal("retries should be 0")
			}

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
