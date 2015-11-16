package main

import (
	"flag"
	"fmt"
	"github.com/gorilla/mux"
	"net/http"
)

var addr = flag.String("addr", ":8081", "http service address")

func main() {
	flag.Parse()
	h := newHub()
	go h.run()
	r := mux.NewRouter()

	// Route websocket requests
	r.Headers("Connection", "Upgrade",
		"Upgrade", "websocket").Handler(wsHandler{h: h})

	// Route other GET and POST requests
	r.Methods("GET").Handler(getHandler{h: h})
	r.Methods("POST").Handler(postHandler{h: h})

	// Start the HTTP server
	http.Handle("/", r)
	fmt.Printf("ListenAndServe(%v, nil)\n", *addr)
	http.ListenAndServe(*addr, nil)
}
