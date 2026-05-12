package relay

import (
	"net"
	"sync"

	"github.com/gorilla/websocket"
)

// TCPWebSocket copies bytes between a TCP connection and a WebSocket connection
// using binary messages. Closes both when either direction ends or errors.
func TCPWebSocket(tcp net.Conn, wsc *websocket.Conn) {
	var once sync.Once
	closeAll := func() {
		once.Do(func() {
			_ = tcp.Close()
			_ = wsc.Close()
		})
	}

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		defer closeAll()
		buf := make([]byte, 32*1024)
		for {
			n, err := tcp.Read(buf)
			if n > 0 {
				if werr := wsc.WriteMessage(websocket.BinaryMessage, buf[:n]); werr != nil {
					return
				}
			}
			if err != nil {
				return
			}
		}
	}()

	go func() {
		defer wg.Done()
		defer closeAll()
		for {
			mt, data, err := wsc.ReadMessage()
			if err != nil {
				return
			}
			if mt != websocket.BinaryMessage {
				continue
			}
			if _, err := tcp.Write(data); err != nil {
				return
			}
		}
	}()

	wg.Wait()
}
