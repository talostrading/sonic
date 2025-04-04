package main

import (
	"fmt"
	"io"
	"net"
	"os"
	"syscall"
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
		n, err := conn.Read(buf)
		if err != nil {
			if err == io.EOF {
				break
			}
			if isConnReset(err) {
				fmt.Println("Client reset connection")
				return
			}
			fmt.Println("Read error:", err)
			return
		}

		buf = buf[:n]
		fmt.Println(string(buf))
		_, err = conn.Write([]byte("1"))
		if err != nil {
			fmt.Println("Write error:", err)
			return
		}
	}

	//fmt.Println("conn", conn.RemoteAddr(), "closed")
}

func isConnReset(err error) bool {
	if opErr, ok := err.(*net.OpError); ok {
		if sysErr, ok := opErr.Err.(*os.SyscallError); ok {
			if sysErr.Err == syscall.ECONNRESET {
				return true
			}
		}
	}
	return false
}
