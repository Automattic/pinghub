package main

import (
	"flag"
	"github.com/facebookgo/httpdown"
	"github.com/gorilla/mux"
	"net/http"
	"time"
)

var origin = flag.String("origin", "", "websocket server checks Origin headers against this scheme://host[:port]")

func main() {
	// Prepare the stoppable HTTP server
	handler := newHandler(*origin)
	http.Handle("/", handler)
	server := &http.Server{
		Addr: "127.0.0.1:8081",
		Handler: handler,
	}
	hd := &httpdown.HTTP{
		StopTimeout: 10 * time.Second,
		KillTimeout: 1 * time.Second,
	}

	// Apply command line arguments
	flag.StringVar(&server.Addr, "addr", server.Addr, "http service address")
	flag.DurationVar(&hd.StopTimeout, "stop-timeout", hd.StopTimeout, "stop timeout")
	flag.DurationVar(&hd.KillTimeout, "kill-timeout", hd.KillTimeout, "kill timeout")
	flag.Parse()

	// Start the server
	if err := httpdown.ListenAndServe(server, hd); err != nil {
		panic(err)
	}
}

func newHandler(origin string) http.Handler {
	hub := newHub()
	go hub.run()

	handler := mux.NewRouter()

	// Route websocket requests
	handler.Headers(
		// Requests with these headers will use this handler
		"Connection", "Upgrade",
		"Upgrade", "websocket",
	).Handler(newWsHandler(hub, origin))

	// Route other GET and POST requests
	handler.Methods("GET").Handler(getHandler{hub: hub})
	handler.Methods("POST").Handler(postHandler{hub: hub})

	return handler
}
