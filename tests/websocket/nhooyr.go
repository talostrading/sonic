package main

import (
	"context"
	"net/http"

	ws "nhooyr.io/websocket"
)

func handler(w http.ResponseWriter, req *http.Request) {
	conn, err := ws.Accept(w, req, &ws.AcceptOptions{
		CompressionMode: ws.CompressionDisabled,
	})
	if err != nil {
		panic(err)
	}
	defer conn.Close(ws.StatusNormalClosure, "")

	ctx := context.Background()
	for {
		t, b, err := conn.Read(ctx)
		if err != nil {
			break
		}
		conn.Write(ctx, t, b)
	}
}

func main() {
	http.HandleFunc("/", handler)
	http.ListenAndServe("127.0.0.1:8080", nil)
}
