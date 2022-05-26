package main

import (
	"net/http"

	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{}

func handler(w http.ResponseWriter, req *http.Request) {
	conn, _, _, err := ws.UpgradeHTTP(req, w)
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	for {
		msg, op, err := wsutil.ReadClientData(conn)
		if err != nil {
			break
		}
		err = wsutil.WriteServerMessage(conn, op, msg)
		if err != nil {
			break
		}
	}
}

func main() {
	http.HandleFunc("/", handler)
	http.ListenAndServe("127.0.0.1:8080", nil)
}
