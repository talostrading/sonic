package main

import (
	"fmt"
	"io"
	"net"
)

func main() {

	ln, err := net.Listen("tcp", ":8080")
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

		go handle(conn)
	}

}

func handle(conn net.Conn) {
	defer conn.Close()

	buf := make([]byte, 4096)
	for {
		//fmt.Println("conn waiting to read")
		n, err := conn.Read(buf)
		if err != nil {
			if err != io.EOF {
				panic(err)
			} else {
				break
			}
		}

		buf = buf[:n]
		fmt.Println(string(buf))
		_, err = conn.Write([]byte("1"))
		if err != nil {
			return
		}
	}

	//fmt.Println("conn", conn.RemoteAddr(), "closed")
}
