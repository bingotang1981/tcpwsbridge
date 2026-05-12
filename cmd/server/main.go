package main

import (
	"flag"
	"log"
	"net/http"
	"os"
	"time"

	"tcpwsbridge/internal/server"
)

func main() {
	listen := flag.String("listen", ":9000", "HTTP listen address")
	path := flag.String("path", "/ws", "WebSocket bridge path")
	token := flag.String("token", "", "if set, require Authorization: Bearer <token>")
	allowAllOrigins := flag.Bool("allow-all-origins", false, "allow any Origin (dev only; unsafe with browsers)")
	dialTimeout := flag.Duration("dial-timeout", 15*time.Second, "timeout for dialing downstream TCP")
	certFile := flag.String("tls-cert", "", "TLS certificate file (optional; enables HTTPS/WSS)")
	keyFile := flag.String("tls-key", "", "TLS key file (optional)")
	flag.Parse()

	bridge := server.Handler(server.Options{
		Token:           *token,
		AllowAllOrigins: *allowAllOrigins,
		DialTimeout:     *dialTimeout,
	})

	mux := http.NewServeMux()
	mux.Handle(*path, bridge)
	mux.HandleFunc("/", server.DefaultPage)

	srv := &http.Server{
		Addr:              *listen,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	log.Printf("bridge server listening on %s path=%s (default page on /)", *listen, *path)

	var err error
	if *certFile != "" || *keyFile != "" {
		if *certFile == "" || *keyFile == "" {
			log.Fatal("both -tls-cert and -tls-key are required for TLS")
		}
		err = srv.ListenAndServeTLS(*certFile, *keyFile)
	} else {
		err = srv.ListenAndServe()
	}

	if err != nil && err != http.ErrServerClosed {
		log.Printf("server: %v", err)
		os.Exit(1)
	}
}
