package main

import (
	"fmt"
	"github.com/gorilla/websocket"
	"io/ioutil"
	"net/http"
	"unicode/utf8"
)

type wsHandler struct {
	h *hub
}

var upgrader = &websocket.Upgrader{ReadBufferSize: 1024, WriteBufferSize: 1024}

func (wsh wsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !validateRequest(w, r) {
		return
	}
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	c := newConnection(ws, wsh.h, r.URL.Path)
	c.run()
}

type getHandler struct {
	h *hub
}

func (gh getHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !validateRequest(w, r) {
		return
	}
	webTemplate.Execute(w, templateArgs{*addr, r.URL.Path})
}

type postHandler struct {
	h *hub
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
	ph.h.queue <- command{cmd: PUBLISH, path: r.URL.Path, text: body}
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
