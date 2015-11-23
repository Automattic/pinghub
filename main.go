package main

import (
	"flag"
	"github.com/gorilla/mux"
	"log"
	"net/http"
)

var addr = flag.String("addr", ":8081", "http service address")
var origin = flag.String("origin", "", "websocket server checks Origin headers against this scheme://host[:port]")

func main() {
	flag.Parse()

	// Start the HTTP server
	http.Handle("/", newHandler(*origin))
	log.Fatal(http.ListenAndServe(*addr, nil))
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
