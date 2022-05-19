package main

import (
	"crypto/tls"
	"flag"
	"fmt"
	"io"
	"net/http"
)

var (
	addr     = flag.String("addr", "https://localhost:8080/", "https server address")
	certPath = flag.String("cert", "./certs/cert.pem", "trusted CA certificate")
	keyPath  = flag.String("key", "./certs/key.pem", "trusted CA certificate")
)

func main() {
	flag.Parse()

	cert, err := tls.LoadX509KeyPair(*certPath, *keyPath)
	if err != nil {
		panic(err)
	}

	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				Certificates:       []tls.Certificate{cert},
				InsecureSkipVerify: true,
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
