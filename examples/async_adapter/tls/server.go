package main

import (
	"crypto/rand"
	"crypto/tls"
	"flag"
	"fmt"
	"io"
	"net"
)

var (
	certPath = flag.String("cert", "./certs/server.pem", "server certificate path")
	keyPath  = flag.String("key", "./certs/server.key", "server private key path")
	addr     = flag.String("addr", ":8080", "tls server address")
)

func main() {
	flag.Parse()

	cert, err := tls.LoadX509KeyPair(*certPath, *keyPath)
	if err != nil {
		panic(err)
	}

	cfg := tls.Config{
		Certificates: []tls.Certificate{cert},
		Rand:         rand.Reader,
	}
	ln, err := tls.Listen("tcp", *addr, &cfg)
	if err != nil {
		panic(err)
	}

	fmt.Println("listening")
	for {
		conn, err := ln.Accept()
		if err != nil {
			conn.Close()
			panic(err)
		}

		fmt.Println("accepted conn", conn.RemoteAddr())

		_, ok := conn.(*tls.Conn)
		if ok {
			fmt.Println("client is using TLS")
		}
		go handle(conn)
	}
}

func handle(conn net.Conn) {
	defer conn.Close()

	buf := make([]byte, 4096)
	for {
		fmt.Println("conn waiting to read")
		n, err := conn.Read(buf)
		if err != nil {
			if err != io.EOF {
				panic(err)
			} else {
				break
			}
		}

		buf = buf[:n]
		fmt.Println("read", string(buf))
		fmt.Println("echoing")

		n, err = conn.Write(buf)
		if err != nil {
			if err != io.EOF {
				panic(err)
			} else {
				break
			}
		}
	}

	fmt.Println("conn", conn.RemoteAddr(), "closed")
}
