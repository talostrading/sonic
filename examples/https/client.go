package main

import (
	"crypto/tls"
	"crypto/x509"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
)

var (
	addr     = flag.String("addr", "https://localhost:8080/", "https server address")
	certfile = flag.String("certfile", "./certs/cert.pem", "trusted CA certificate")
)

func main() {
	flag.Parse()

	cert, err := os.ReadFile(*certfile)
	if err != nil {
		panic(err)
	}

	certPool := x509.NewCertPool()
	if ok := certPool.AppendCertsFromPEM(cert); !ok {
		panic(fmt.Sprintf("unable to parse certificate from %s", *certfile))
	}

	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				RootCAs: certPool,
			},
		},
	}

	res, err := client.Get(*addr)
	if err != nil {
		panic(err)
	}
	defer res.Body.Close()

	if res.StatusCode == http.StatusOK {
		body, err := io.ReadAll(res.Body)
		if err != nil {
			panic(err)
		}
		fmt.Println("response:", string(body))
	} else {
		panic(fmt.Sprintf("status_code: %d", res.StatusCode))
	}
}
