package main

import (
	"crypto/tls"
	"flag"
	"net/http"
)

var (
	addr     = flag.String("addr", ":8080", "https network address")
	certFile = flag.String("cert", "cert.pem", "certificate PEM file")
	keyFile  = flag.String("key", "key.pem", "private key PEM file")
)

func main() {
	flag.Parse()

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path != "/" {
			http.NotFound(w, req)
			return
		}
	})

	srv := &http.Server{
		Addr:    *addr,
		Handler: mux,
		TLSConfig: &tls.Config{
			MinVersion:               tls.VersionTLS13,
			PreferServerCipherSuites: true,
		},
	}

	println("listening")
	if err := srv.ListenAndServeTLS(*certFile, *keyFile); err != nil {
		panic(err)
	}
}
