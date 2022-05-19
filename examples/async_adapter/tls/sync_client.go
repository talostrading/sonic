package main

import (
	"crypto/tls"
	"flag"
	"fmt"
	"io"
)

var (
	certPath = flag.String("cert", "./certs/client.pem", "client certificate path")
	keyPath  = flag.String("key", "./certs/client.key", "client private key path")
	addr     = flag.String("addr", ":8080", "tls server address")
)

func main() {
	cert, err := tls.LoadX509KeyPair(*certPath, *keyPath)
	if err != nil {
		panic(err)
	}

	cfg := tls.Config{
		Certificates:       []tls.Certificate{cert},
		InsecureSkipVerify: true,
	}

	conn, err := tls.Dial("tcp", *addr, &cfg)
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	fmt.Println("client connected to", conn.RemoteAddr())

	state := conn.ConnectionState()
	fmt.Println("handshake", state.HandshakeComplete)

	_, err = io.WriteString(conn, "hello")
	if err != nil {
		panic(err)
	}

	buf := make([]byte, 4096)
	n, err := conn.Read(buf)
	if err != nil {
		panic(err)
	}
	buf = buf[:n]
	fmt.Println("client read", string(buf))
	fmt.Println("bye")
}
