package main

import (
	"fmt"
	"net"
	"time"

	"github.com/talostrading/sonic"
	"github.com/talostrading/sonic/sonicopts"
)

func main() {
	go func() {
		ln, err := net.Listen("tcp", "localhost:8080")
		if err != nil {
			panic(err)
		}
		conn, err := ln.Accept()
		if err != nil {
			panic(err)
		}
		t := time.NewTimer(200 * time.Millisecond)
		<-t.C
		println("sending hello1")
		_, err = conn.Write([]byte("hello1"))
		if err != nil {
			panic(err)
		}
		t = time.NewTimer(400 * time.Millisecond)
		<-t.C
		println("sending hello2")
		_, err = conn.Write([]byte("hello2"))
		if err != nil {
			panic(err)
		}
		t = time.NewTimer(600 * time.Millisecond)
		<-t.C
		println("sending hello3")
		_, err = conn.Write([]byte("hello3"))
		if err != nil {
			panic(err)
		}
	}()
	ioc := sonic.MustIO()
	defer ioc.Close()
	conn, err := sonic.DialTimeout(ioc, "tcp", "localhost:8080", time.Second*5, sonicopts.Nonblocking(false))
	if err != nil {
		panic(err)
	}

	counter := 0
	b := make([]byte, 128)
	for {
		b = b[:cap(b)]
		n, err := conn.Read(b)
		b := b[:n]
		fmt.Println(n, err, string(b))
		if err == nil {
			counter++
		}
		if counter == 3 {
			break
		}
	}

}
