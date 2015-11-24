package main

import (
	"flag"
	"github.com/facebookgo/httpdown"
	"github.com/gorilla/mux"
	"net/http"
	"time"
)


func main() {
	// Prepare the stoppable HTTP server
	server := &http.Server{
		Addr: "127.0.0.1:8081",
	}
	hd := &httpdown.HTTP{
		StopTimeout: 10 * time.Second,
		KillTimeout: 1 * time.Second,
	}

	flag.StringVar(&server.Addr, "addr", server.Addr, "http service address")
	flag.DurationVar(&hd.StopTimeout, "stop-timeout", hd.StopTimeout, "stop timeout")
	flag.DurationVar(&hd.KillTimeout, "kill-timeout", hd.KillTimeout, "kill timeout")
	origin := flag.String("origin", "", "websocket server checks Origin headers against this scheme://host[:port]")
	flag.Parse()

	// Start the server
	server.Handler = newHandler(*origin)
	http.Handle("/", server.Handler)
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
