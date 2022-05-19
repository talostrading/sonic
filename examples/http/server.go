package main

import (
	"fmt"
	"io"
	"net/http"
)

func handler(res http.ResponseWriter, req *http.Request) {
	buf, err := io.ReadAll(req.Body)
	if err != nil {
		panic("error")
	}
	fmt.Println("received:", string(buf))
	fmt.Fprintf(res, "hello")
}

func main() {
	http.HandleFunc("/", handler)
	http.ListenAndServe(":8080", nil)
}
