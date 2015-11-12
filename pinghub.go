// Package pinghub serves as a simple message bus (pubsub) over websockets.
//
//     pinghub -addr=:8081
//
// Everything is as ephemeral as can be. A message is sent to connected
// subscribers (if any) and then forgotten. A channel is forgotten when
// its last subscriber disconnects.
//
// Subscribe to a channel by opening a websocket to a valid path.
//     ws://localhost:8081/Path_must_be_valid_UTF-8
//
// Publish by sending a plain text websocket message (valid UTF-8).
//
// Publish by POSTing to the same path with a plain text body.
//     curl localhost:8081/Path_must_be_valid_UTF-8 -d "Hello"
//
// Messages are sent to all subscribers connected to the path, regardless
// of whether they were also the sender.
//
// Paths and messages must be valid UTF-8. Paths can be 1-256 characters.
// Message length should be limited but it is not.
//
// Non-websocket GET requests are served HTML with a websocket client that
// connects to the requested path.
//     http://localhost:8081/Path_must_be_valid_UTF-8
package main

import (
	"flag"
	"fmt"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"html/template"
	"io/ioutil"
	"net/http"
	"unicode/utf8"
)

const (
	pathLenMin = 1
	pathLenMax = 256
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
	http.ListenAndServe(*addr, nil)
}

type envelope struct {
	path string
	text string
}

type hub struct {
	connections map[string]map[*connection]bool
	register    chan *connection
	unregister  chan *connection
	messages    chan *envelope
}

func newHub() *hub {
	return &hub{
		connections: make(map[string]map[*connection]bool),
		register:    make(chan *connection),
		unregister:  make(chan *connection),
		messages:    make(chan *envelope),
	}
}

func (h *hub) run() {
	for {
		select {
		case c := <-h.register:
			h.addConn(c)
		case c := <-h.unregister:
			h.removeConn(c)
		case m := <-h.messages:
			if conns, ok := h.connections[m.path]; ok {
				for c := range conns {
					select {
					case c.send <- []byte(m.text):
					default:
						h.removeConn(c)
					}
				}
			}
		}
	}
}

func (h *hub) addConn(c *connection) {
	if _, ok := h.connections[c.path]; !ok {
		h.connections[c.path] = make(map[*connection]bool)
	}
	h.connections[c.path][c] = true
}

func (h *hub) removeConn(c *connection) {
	if _, ok := h.connections[c.path][c]; !ok {
		return
	}
	delete(h.connections[c.path], c)
	if len(h.connections[c.path]) == 0 {
		delete(h.connections, c.path)
	}
	close(c.send)
}

type connection struct {
	ws   *websocket.Conn
	send chan []byte
	h    *hub
	path string
}

func (c *connection) reader() {
	for {
		_, message, err := c.ws.ReadMessage()
		if err != nil {
			break
		}
		c.h.messages <- &envelope{c.path, string(message)}
	}
	c.ws.Close()
}

func (c *connection) writer() {
	for message := range c.send {
		err := c.ws.WriteMessage(websocket.TextMessage, message)
		if err != nil {
			break
		}
	}
	c.ws.Close()
}

var upgrader = &websocket.Upgrader{ReadBufferSize: 1024, WriteBufferSize: 1024}

type wsHandler struct {
	h *hub
}

func (wsh wsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !validateRequest(w, r) {
		return
	}
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	c := &connection{
		send: make(chan []byte, 256),
		ws:   ws,
		h:    wsh.h,
		path: r.URL.Path}
	c.h.register <- c
	defer func() { c.h.unregister <- c }()
	go c.writer()
	c.reader()
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

type getHandler struct {
	h *hub
}

func (gh getHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if ! validateRequest(w, r) {
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
	ph.h.messages <- &envelope{r.URL.Path, string(body)}
	w.Write([]byte("OK\n"))
}

type templateArgs struct {
	Addr, Path string
}

var webTemplate = template.Must(template.New("webTemplate").Parse(`
<html>
<head>
<title>pinghub {{.Path}}</title>
<script type="text/javascript" src="http://ajax.googleapis.com/ajax/libs/jquery/1.4.2/jquery.min.js"></script>
<script type="text/javascript">
    $(function() {

    var conn;
    var msg = $("#msg");
    var log = $("#log");

    function appendLog(msg) {
        var d = log[0]
        var doScroll = d.scrollTop == d.scrollHeight - d.clientHeight;
        msg.appendTo(log)
        if (doScroll) {
            d.scrollTop = d.scrollHeight - d.clientHeight;
        }
    }

    $("#form").submit(function() {
        if (!conn) {
            return false;
        }
        if (!msg.val()) {
            return false;
        }
        conn.send(msg.val());
        msg.val("");
        return false
    });

    if (window["WebSocket"]) {
        conn = new WebSocket("ws://localhost{{.Addr}}{{.Path}}");
        conn.onclose = function(evt) {
            appendLog($("<div><b>Connection closed.</b></div>"))
        }
        conn.onmessage = function(evt) {
            appendLog($("<div/>").text(evt.data))
        }
        msg.focus();
    } else {
        appendLog($("<div><b>Your browser does not support WebSockets.</b></div>"))
    }
    });
</script>
<style type="text/css">
html {
    overflow: hidden;
}

body {
    overflow: hidden;
    padding: 0.5em;
    margin: 0;
    width: 100%;
    height: 100%;
    background: gray;
}

#log {
    background: white;
    margin: 0;
    padding: 0.5em 0.5em 0.5em 0.5em;
    position: absolute;
    top: 2.0em;
    left: 0.5em;
    right: 0.5em;
    bottom: 3em;
    overflow: auto;
}

#form {
    padding: 0 0.5em 0 0.5em;
    margin: 0;
    position: absolute;
    bottom: 0.5em;
    left: 0px;
    width: 100%;
    overflow: hidden;
}

</style>
</head>
<body>
<h3>Websocket client for {{.Path}}</h3>
<div id="log"></div>
<form id="form">
    <input type="submit" value="Send" />
    <input type="text" id="msg" size="64"/>
</form>
</body>
</html>
`))
