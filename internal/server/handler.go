package server

import (
	"log"
	"net"
	"net/http"
	"strconv"
	"time"

	"tcpwsbridge/internal/relay"
	"tcpwsbridge/internal/target"

	"github.com/gorilla/websocket"
)

// Options configures the WebSocket bridge HTTP handler.
type Options struct {
	Token           string
	AllowAllOrigins bool
	DialTimeout     time.Duration
}

// Handler returns an http.Handler for the WebSocket bridge path only.
// Non-WebSocket HTTP requests receive the same default landing page as [DefaultPage].
func Handler(opts Options) http.Handler {
	if opts.DialTimeout <= 0 {
		opts.DialTimeout = 15 * time.Second
	}

	up := websocket.Upgrader{
		ReadBufferSize:  32 * 1024,
		WriteBufferSize: 32 * 1024,
		// Origin policy is enforced above before Upgrade; allow library handshake once we pass checks.
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !websocket.IsWebSocketUpgrade(r) {
			DefaultPage(w, r)
			return
		}

		if opts.Token != "" {
			want := "Bearer " + opts.Token
			if r.Header.Get("Authorization") != want {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
		}

		if !opts.AllowAllOrigins {
			if origin := r.Header.Get("Origin"); origin != "" {
				http.Error(w, "origin not allowed", http.StatusForbidden)
				return
			}
		}

		host, port, err := target.FromQuery(r.URL.Query())
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		addr := net.JoinHostPort(host, strconv.Itoa(port))

		wsConn, err := up.Upgrade(w, r, nil)
		if err != nil {
			log.Printf("websocket upgrade: %v", err)
			return
		}

		tcpConn, err := net.DialTimeout("tcp", addr, opts.DialTimeout)
		if err != nil {
			log.Printf("dial downstream %s: %v", addr, err)
			_ = wsConn.Close()
			return
		}

		log.Printf("relay start remote=%s downstream=%s", r.RemoteAddr, addr)
		relay.TCPWebSocket(tcpConn, wsConn)
		log.Printf("relay end remote=%s downstream=%s", r.RemoteAddr, addr)
	})
}
