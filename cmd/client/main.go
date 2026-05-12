package main

import (
	"context"
	"flag"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"sync/atomic"
	"syscall"
	"time"

	"tcpwsbridge/internal/relay"

	"github.com/gorilla/websocket"
)

func main() {
	listen := flag.String("listen", ":8080", "local TCP listen address")
	bridge := flag.String("bridge", "ws://127.0.0.1:9000/ws", "bridge server WebSocket URL; downstream target is added as query params h and p")
	downHost := flag.String("host", "127.0.0.1", "downstream TCP host for bridge to dial")
	downPort := flag.Int("port", 0, "downstream TCP port for bridge to dial (required)")
	token := flag.String("token", "", "if set, send Authorization: Bearer <token>")
	maxConns := flag.Int("max-conns", 0, "maximum concurrent upstream TCP connections (0 = unlimited)")
	dialTimeout := flag.Duration("dial-timeout", 15*time.Second, "timeout for WebSocket dial to bridge")
	flag.Parse()

	if *downPort < 1 || *downPort > 65535 {
		log.Fatal("-port must be between 1 and 65535")
	}

	wsURL, err := buildBridgeURL(*bridge, *downHost, *downPort)
	if err != nil {
		log.Fatalf("bridge URL: %v", err)
	}

	ln, err := net.Listen("tcp", *listen)
	if err != nil {
		log.Fatalf("listen: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		<-ctx.Done()
		_ = ln.Close()
	}()

	log.Printf("client TCP listening on %s -> bridge %s (downstream %s:%d)", *listen, redactURL(wsURL), *downHost, *downPort)

	var active int64
	for {
		conn, err := ln.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				return
			default:
				log.Printf("accept: %v", err)
				continue
			}
		}

		if *maxConns > 0 && int(atomic.LoadInt64(&active)) >= *maxConns {
			log.Printf("max connections reached, rejecting %s", conn.RemoteAddr())
			_ = conn.Close()
			continue
		}

		atomic.AddInt64(&active, 1)
		go func(c net.Conn) {
			defer atomic.AddInt64(&active, -1)
			handleUpstream(ctx, c, wsURL.String(), *token, *dialTimeout)
		}(conn)
	}
}

func redactURL(u *url.URL) string {
	if u == nil {
		return ""
	}
	cp := *u
	cp.User = nil
	return cp.String()
}

func buildBridgeURL(bridge, host string, port int) (*url.URL, error) {
	u, err := url.Parse(bridge)
	if err != nil {
		return nil, err
	}
	q := u.Query()
	q.Set("h", host)
	q.Set("p", strconv.Itoa(port))
	u.RawQuery = q.Encode()
	return u, nil
}

func handleUpstream(ctx context.Context, tcpConn net.Conn, wsURL string, token string, dialTimeout time.Duration) {
	defer tcpConn.Close()

	dctx, cancel := context.WithTimeout(ctx, dialTimeout)
	defer cancel()

	hdr := http.Header{}
	if token != "" {
		hdr.Set("Authorization", "Bearer "+token)
	}

	dialer := websocket.Dialer{
		HandshakeTimeout: dialTimeout,
	}

	wsConn, resp, err := dialer.DialContext(dctx, wsURL, hdr)
	if err != nil {
		if resp != nil {
			log.Printf("websocket dial failed: %v (HTTP %s)", err, resp.Status)
		} else {
			log.Printf("websocket dial failed: %v", err)
		}
		return
	}

	log.Printf("tunnel %s <-> bridge", tcpConn.RemoteAddr())
	relay.TCPWebSocket(tcpConn, wsConn)
	log.Printf("tunnel closed %s", tcpConn.RemoteAddr())
}
