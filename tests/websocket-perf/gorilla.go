package main

import (
	"net/http"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{}

func handler(w http.ResponseWriter, req *http.Request) {
	conn, err := upgrader.Upgrade(w, req, nil)
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	for {
		t, b, err := conn.ReadMessage()
		if err != nil {
			break
		}
		conn.WriteMessage(t, b)
	}
}

func main() {
	http.HandleFunc("/", handler)
	http.ListenAndServe("127.0.0.1:8080", nil)
}
