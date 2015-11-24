package main

import (
	"fmt"
	"github.com/gorilla/websocket"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"unicode/utf8"
)

type wsHandler struct {
	hub      *hub
	upgrader *websocket.Upgrader
}

func newWsHandler(hub *hub, origin string) wsHandler {
	return wsHandler{
		hub: hub,
		upgrader: &websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin:     wsOriginChecker(origin),
		},
	}
}

func wsOriginChecker(origin string) func(r *http.Request) bool {
	if origin == "" {
		return func(r *http.Request) bool {
			return true
		}
	}

	serverOriginURL, err := url.Parse(origin)
	if err != nil {
		log.Fatal("Failed to parse origin", origin, err)
	}
	return func(r *http.Request) bool {
		o := r.Header["Origin"]
		if len(o) == 0 {
			return true
		}
		clientOrigin := o[0]
		if clientOrigin == origin {
			return true
		}
		clientOriginURL, err := url.Parse(clientOrigin)
		if err != nil {
			return false
		}
		if clientOriginURL.Scheme != serverOriginURL.Scheme {
			return false
		}
		if clientOriginURL.Host != serverOriginURL.Host {
			return false
		}
		return true
	}
}

func (wsh wsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !validateRequest(w, r) {
		return
	}
	ws, err := wsh.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	c := newConnection(ws, wsh.hub, r.URL.Path)
	c.run()
}

type getHandler struct {
	hub *hub
}

func (gh getHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !validateRequest(w, r) {
		return
	}
	webTemplate.Execute(w, templateArgs{r.URL.Path})
}

type postHandler struct {
	hub *hub
}

func (ph postHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !validateRequest(w, r) {
		return
	}
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		sendBadRequestError(w, "Unable to read POST body.")
		return
	}
	ph.hub.queue <- command{cmd: PUBLISH, path: r.URL.Path, text: body}
	w.Write([]byte("OK\n"))
}

func validateRequest(w http.ResponseWriter, r *http.Request) bool {
	if !utf8.ValidString(r.URL.Path) {
		sendBadRequestError(w, "Path must be valid Unicode (UTF-8).")
		return false
	}
	pathLen := utf8.RuneCountInString(r.URL.Path)
	if !(pathLenMin <= pathLen && pathLen <= pathLenMax) {
		sendBadRequestError(w, fmt.Sprintf(
			"Path length must be %d-%d Unicode characters (UTF-8).",
			pathLenMin, pathLenMax))
		return false
	}
	return true
}

func sendBadRequestError(w http.ResponseWriter, str string) {
	http.Error(w,
		fmt.Sprintf("Error: bad request. %s", str),
		http.StatusBadRequest)
}
