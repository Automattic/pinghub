package main

import (
	"flag"
	"github.com/gorilla/mux"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
)

func main() {
	// Prepare the HTTP server
	server := &http.Server{
		Addr: "127.0.0.1:8081",
	}

	metricsPort := "8082"
	flag.StringVar(&metricsPort, "mport", metricsPort, "metrics service port")
	flag.StringVar(&server.Addr, "addr", server.Addr, "http service address (TCP address or absolute path for UNIX socket)")
	origin := flag.String("origin", "", "websocket server checks Origin headers against this scheme://host[:port]")
	logpath := flag.String("log", "", "Log file (absolute path)");

	flag.Parse()

	if strings.HasPrefix(*logpath, "/") {
		logf, err := os.OpenFile(*logpath, os.O_RDWR | os.O_CREATE | os.O_APPEND, 0666)
		if err != nil {
			log.Fatalf("error opening log file: %v", err)
		}
		defer func(){
			log.Printf( "********** pid %d stopping **********", os.Getpid())
			logf.Close()
		}()
		if err = syscall.Dup2(int(logf.Fd()), syscall.Stdout); err != nil {
			log.Fatalf("error redirecting stdout to file: %v", err)
		}
		if err = syscall.Dup2(int(logf.Fd()), syscall.Stderr); err != nil {
			log.Fatalf("error redirecting stderr to file: %v", err)
		}
		log.SetFlags(log.Ldate | log.Lmicroseconds | log.LUTC)
		log.Printf( "********** pid %d starting **********", os.Getpid())
	}

	// Initialize metrics registry with expected stats
	go startMetrics(metricsPort)
	incr("websockets", 0)    // number of connected websockets
	incr("channels", 0)      // number of subscribed channels
	mark("postmsgs", 0)      // rate of POST messages
	mark("websocketmsgs", 0) // rate of WS messages
	mark("drops", 0)         // rate of messages sent to nobody
	mark("sends", 0)         // rate of messages sent to somebody

	// Start the server
	server.Handler = newHandler(*origin)
	http.Handle("/", server.Handler)
	if strings.HasPrefix(server.Addr, "/") {
		ln, err := net.Listen("unix", server.Addr)
		if err != nil {
			panic(err)
		}
		closeListenerOnSignals(ln)
		server.Serve(ln)
	} else {
		server.ListenAndServe()
	}
}

func closeListenerOnSignals(ln net.Listener) {
	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, os.Interrupt, os.Kill, syscall.SIGTERM)
	go func(c chan os.Signal) {
		<-c
		ln.Close()
		os.Exit(0)
	}(sigc)
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
