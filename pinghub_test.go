package main

import (
	"github.com/gorilla/websocket"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httptest"
	"net/URL"
	"os"
	"strings"
	"testing"
	"time"
)

const (
	WS = 0
	POST = 1
)

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}

func TestApp(t *testing.T) {
	var url *url.URL
	var resp *http.Response
	var body string
	var err error

	t.Log("Test: start server")
	var server *httptest.Server = mockApp()
	defer server.Close()
	_, err = url.Parse(server.URL)
	if err != nil {
		t.Fatal("Server URL parse error:", err)
	}

	t.Log("Test: GET /somestring serves HTML containing /somestring")
	url, _ = url.Parse(server.URL)
	url.Path = "/somestring"
	resp = get(t, url)
	body = string(responseBody(t, resp))
	if ! strings.Contains(body, "<html>") {
		t.Fatal("No HTML from server:", resp)
	}
	if ! strings.Contains(body, "/somestring") {
		t.Fatal("Path not found in HTML:", resp)
	}

	t.Log("Test: GET /<xss> does not return <xss>")
	url, _ = url.Parse(server.URL)
	url.Path = "/<xss>"
	resp = get(t, url)
	body = string(responseBody(t, resp))
	if strings.Contains(body, "<xss>") {
		t.Fatal("HTML contains <xss>")
	}

	t.Log("Test: clients connect and send messages")
	var hub = mockHub()
	var clients = map[string][]*client {
		"/path1": {
			mockClient("A", WS),
			mockClient("B", WS),
			mockClient("C", POST),
		},
		"/path2": {
			mockClient("D", POST),
			mockClient("E", WS),
			mockClient("F", WS),
		},
		"/path3": {
			mockClient("G", WS),
			mockClient("H", POST),
			mockClient("I", POST),
			mockClient("J", WS),
		},
	}

	for path := range clients {
		for i := range clients[path] {
			c := clients[path][i]
			url, _ = url.Parse(server.URL)
			url.Path = path
			switch c.method {

			case WS:
				url.Scheme = "ws"
				c.ws = mockWs(t, url, c)
				hub.subscribe(path, c)
				defer c.ws.Close()
				go c.run(t)
				c.sendSync(c.message)
				hub.send(path, c.message)

			case POST:
				resp := post(t, url, c.message)
				if resp.Status != "200 OK" || string(responseBody(t, resp)) != "OK\n" {
					t.Fatal("POST response not 200 OK:", resp)
				}
				hub.send(path, c.message)
			}
		}
	}

	t.Log("Test: websocket clients receive messages in order")
	for path := range clients {
		for i := range clients[path] {
			c := clients[path][i]
			if c.method == WS {
				for _, expected := range hub.receiveAll(path, c) {
					message := c.receiveSync()
					if message != expected {
						t.Fatal("expected", expected, "got", message)
					}
				}
				c.stop()
			}
		}
	}
}

type fakeHub struct {
	m map[string]map[*client]chan string
}

func mockHub() *fakeHub {
	return &fakeHub{
		m: make(map[string]map[*client]chan string),
	}
}

func (h *fakeHub) subscribe(path string, c *client) {
	if _, ok := h.m[path]; !ok {
		h.m[path] = make(map[*client]chan string)
	}
	h.m[path][c] = make(chan string, 1000)
}

func (h *fakeHub) send(path string, message string) {
	if _, ok := h.m[path]; !ok {
		return
	}
	for c := range h.m[path] {
		h.m[path][c]<- message
	}
}

func (h *fakeHub) receiveAll(path string, client *client) (messages []string) {
	for { select {
	case m := <-h.m[path][client]:
		messages = append(messages, m)
	default:
		return
	}}
}

func mockApp() *httptest.Server {
	return httptest.NewServer(newHandler())
}

func get(t *testing.T, url *url.URL) *http.Response {
	resp, err := http.Get(url.String())
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func responseBody(t *testing.T, r *http.Response) []byte {
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		t.Fatal(err)
	}
	return body
}

func post(t *testing.T, url *url.URL, message string) *http.Response {
	resp, err := http.Post(url.String(), "text/plain", strings.NewReader(message))
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

type client struct {
	message string
	method int
	ws *websocket.Conn
	tx chan string
	rx chan bool
	res chan string
	end chan bool
}

func mockClient(message string, method int) *client {
	if method == POST {
		return &client{message, method, nil, nil, nil, nil, nil}
	}
	return &client{
		message,
		method,
		nil,
		make(chan string, 1),
		make(chan bool, 1),
		make(chan string, 1),
		make(chan bool, 1),
	}
}

func (c *client) run(t *testing.T) {
	for { select {
	case message := <-c.tx :
		err := c.ws.WriteMessage(
			websocket.TextMessage, []byte(message))
		if err != nil {
			t.Fatal("WriteMessage:", err)
		}
		c.res<- "OK"
	case <-c.rx :
		_, message, err := c.ws.ReadMessage()
		if err != nil {
			t.Fatal("ReadMessage:", err)
		}
		c.res<- string(message)
	case <-c.end:
		break
	}}
}

func (c *client) sendSync(message string) {
	c.tx<- message
	<-c.res
}

func (c *client) receiveSync() string {
	c.rx<- true
	message := <-c.res
	return string(message)
}

func (c *client) stop() {
	c.end<- true
}

func mockWs(t *testing.T, url *url.URL, c *client) *websocket.Conn {
	dialer := &websocket.Dialer{
		NetDial: func(network, addr string) (net.Conn, error) {
			d := net.Dialer{
				Timeout: 3 * time.Second,
			}
			return d.Dial(network, url.Host) },
		Proxy: http.ProxyFromEnvironment,
		HandshakeTimeout: 3 * time.Second,
		Subprotocols: []string{"p1", c.message},
	}
	ws, resp, err := dialer.Dial(url.String(), nil)
	if err != nil {
		t.Fatal("dial error:", err, "resp:", resp)
	}
	return ws
}
