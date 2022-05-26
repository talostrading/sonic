package main

import (
	"fmt"
	"net/http"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{}

func handler(w http.ResponseWriter, r *http.Request) {
	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		panic(err)
	}
	defer c.Close()

	for {
		msgType, buf, err := c.ReadMessage()
		if err != nil {
			panic(err)
		}
		fmt.Println("read message type:", msgType, "msg:", string(buf))

		err = c.WriteMessage(websocket.TextMessage, buf)
		if err != nil {
			panic(err)
		}
	}
}

func main() {
	http.HandleFunc("/", handler)
	http.ListenAndServe("localhost:8080", nil)
}
