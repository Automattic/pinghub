package main

import (
	"github.com/gorilla/websocket"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"
)

const (
	WS   = 0
	POST = 1
)

var server *httptest.Server

func TestMain(m *testing.M) {
	os.Exit(runServer(m))
}

func runServer(m *testing.M) int {
	server = httptest.NewServer(newHandler())
	defer server.Close()
	_, err := url.Parse(server.URL)
	if err != nil {
		log.Fatal("Server URL parse error:", err)
	}
	return m.Run()
}

func TestHTML(t *testing.T) {
	t.Log("TestHTML: GET /somestring serves HTML containing /somestring")
	u, _ := url.Parse(server.URL)
	u.Path = "/somestring"
	resp := get(t, u)
	body := string(responseBody(t, resp))
	if !strings.Contains(body, "<html>") {
		t.Fatal("No HTML from server:", resp)
	}
	if !strings.Contains(body, "/somestring") {
		t.Fatal("Path not found in HTML:", resp)
	}
}

func TestXSS(t *testing.T) {
	t.Log("TestXSS: GET /<xss> does not return <xss>")
	u, _ := url.Parse(server.URL)
	u.RawPath = "/<xss>"
	resp := get(t, u)
	body := string(responseBody(t, resp))
	if strings.Contains(body, "<xss>") {
		t.Fatal("HTML contains <xss>")
	}
}

func TestClients(t *testing.T) {
	t.Log("TestClients: clients connect and publish messages")
	hub := mockHub()
	clients := map[string][]*client{
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
			mockClient("K", WS),
			mockClient("L", POST),
			mockClient("M", POST),
			mockClient("N", WS),
		},
	}

	for path := range clients {
		for i := range clients[path] {
			c := clients[path][i]
			u, _ := url.Parse(server.URL)
			u.Path = path
			switch c.method {

			case WS:
				u.Scheme = "ws"
				c.ws = mockWs(t, u, c)
				defer c.ws.Close()
				hub.subscribe(path, c)
				go c.reader()
				c.sendSync(t, c.message)
				hub.send(path, c.message)

			case POST:
				resp := post(t, u, c.message)
				if resp.Status != "200 OK" || string(responseBody(t, resp)) != "OK\n" {
					t.Fatal("POST response not 200 OK:", resp)
				}
				hub.send(path, c.message)
			}
		}
	}

	t.Log("TestClients: clients receive messages in order")
	// Give the server some time to transact with all clients.
	time.Sleep(50 * time.Millisecond)
	for path := range clients {
		for i := range clients[path] {
			c := clients[path][i]
			if c.method == WS {
				expected := strings.Join(hub.receiveAll(path, c), "")
				got := c.readAll()
				if expected != got {
					t.Fatal("expected", expected, "got", got)
				}
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
		h.m[path][c] <- message
	}
}

func (h *fakeHub) receiveAll(path string, client *client) (messages []string) {
	for {
		select {
		case m := <-h.m[path][client]:
			messages = append(messages, m)
		default:
			return
		}
	}
}

func get(t *testing.T, u *url.URL) *http.Response {
	resp, err := http.Get(u.String())
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

func post(t *testing.T, u *url.URL, message string) *http.Response {
	resp, err := http.Post(u.String(), "text/plain", strings.NewReader(message))
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

type client struct {
	message string
	method  int
	waiting bool
	ws      *websocket.Conn
	res     chan struct{}
	rec     []string
}

func mockClient(message string, method int) *client {
	if method == POST {
		return &client{message, method, false, nil, nil, nil}
	}
	return &client{
		message,
		method,
		false,
		nil,
		make(chan struct{}),
		[]string{},
	}
}

func (c *client) reader() {
	for {
		_, message, err := c.ws.ReadMessage()
		if err != nil {
			return
		}
		c.rec = append(c.rec, string(message))
		if c.waiting {
			c.res <- struct{}{}
		}
	}
}

// Send a message and block until echo is received
func (c *client) sendSync(t *testing.T, message string) {
	c.waiting = true
	err := c.ws.WriteMessage(
		websocket.TextMessage, []byte(message))
	if err != nil {
		t.Fatal("WriteMessage:", err)
	}
	_, ok := <-c.res
	if !ok {
		t.Fatal("Failed waiting for echo.")
	}
	c.waiting = false
}

func (c *client) readAll() string {
	return strings.Join(c.rec, "")
}

func mockWs(t *testing.T, u *url.URL, c *client) *websocket.Conn {
	dialer := &websocket.Dialer{
		NetDial: func(network, addr string) (net.Conn, error) {
			d := net.Dialer{
				Timeout: 3 * time.Second,
			}
			return d.Dial(network, u.Host)
		},
		Proxy:            http.ProxyFromEnvironment,
		HandshakeTimeout: 3 * time.Second,
		Subprotocols:     []string{"p1", c.message},
	}
	ws, resp, err := dialer.Dial(u.String(), nil)
	if err != nil {
		t.Fatal("dial error:", err, "resp:", resp)
	}
	return ws
}
