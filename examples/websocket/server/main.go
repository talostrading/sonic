package main

import (
	"fmt"

	"github.com/talostrading/sonic"
	"github.com/talostrading/sonic/codec/websocket"
	"github.com/talostrading/sonic/sonicopts"
)

func main() {
	ioc := sonic.MustIO()
	defer ioc.Close()

	ln, err := sonic.Listen(ioc, "tcp", "localhost:8080", sonicopts.Nonblocking(true))
	if err != nil {
		panic(err)
	}
	defer ln.Close()

	fmt.Println("WebSocket server listening on ws://localhost:8080")

	var onAccept sonic.AcceptCallback
	onAccept = func(err error, conn sonic.Conn) {
		if err != nil {
			fmt.Println("accept error:", err)
			return
		}

		// Accept the next connection
		ln.AsyncAccept(onAccept)

		// Create a new WebSocket stream for this connection
		ws, err := websocket.NewWebsocketStream(ioc, nil, websocket.RoleServer)
		if err != nil {
			fmt.Println("failed to create websocket stream:", err)
			conn.Close()
			return
		}

		// Perform the WebSocket handshake
		ws.AsyncAccept(conn, func(err error) {
			if err != nil {
				fmt.Println("handshake error:", err)
				ws.CloseNextLayer()
				return
			}

			fmt.Println("client connected from", conn.RemoteAddr())

			// Start reading messages
			handleConnection(ws)
		})
	}

	ln.AsyncAccept(onAccept)

	ioc.Run()
}

func handleConnection(ws *websocket.Stream) {
	b := make([]byte, 512)

	var onMessage websocket.AsyncMessageCallback
	onMessage = func(err error, n int, mt websocket.MessageType) {
		if err != nil {
			fmt.Println("read error:", err)
			ws.Close(websocket.CloseNormal, "goodbye")
			return
		}

		msg := b[:n]
		fmt.Printf("received [%s]: %s\n", mt, string(msg))

		// Echo the message back
		ws.AsyncWrite(msg, mt, func(err error) {
			if err != nil {
				fmt.Println("write error:", err)
				ws.Close(websocket.CloseNormal, "goodbye")
				return
			}

			fmt.Println("echoed message back")

			// Continue reading
			ws.AsyncNextMessage(b, onMessage)
		})
	}

	ws.AsyncNextMessage(b, onMessage)
}
