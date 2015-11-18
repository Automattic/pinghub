package main

import (
	"fmt"
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

var _ = fmt.Sprintf
var _ = os.Exit

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
	var messages = make(map[string][]string)
	const (
		WS = 0
		POST = 1
	)
	var clients = map[string][]*client {
		"/path1": {			
			{"A", WS, nil, make(chan string, 1), make(chan bool, 1), make(chan string, 1)},
//			{"B", WS, nil, make(chan string, 1), make(chan bool, 1), make(chan string, 1)},
			{"C", POST, nil, nil, nil, nil}},
		"/path2": {			
			{"D", WS, nil, make(chan string, 1), make(chan bool, 1), make(chan string, 1)},
//			{"E", WS, nil, make(chan string, 1), make(chan bool, 1), make(chan string, 1)},
			{"F", POST, nil, nil, nil, nil}},
	}

	for path, cs := range clients {
		messages[path] = []string{}
		for i, c := range cs {
			messages[path] = append(messages[path], c.message)
			url, _ = url.Parse(server.URL)
			url.Path = path
			switch c.method {

			case WS:
				url.Scheme = "ws"
				c.ws = mockWsClient(t, url, c)
				defer c.ws.Close()
				go c.run(t)
				c.send(c.message)
				clients[path][i] = c

			case POST:
				resp := post(t, url, c.message)
				if resp.Status != "200 OK" || string(responseBody(t, resp)) != "OK\n" {
					t.Fatal("POST response not 200 OK:", resp)
				}
				fmt.Println("POST", c.message)
			}
		}
	}

	t.Log("Test: websocket clients receive messages in order")
	for path, cs := range clients {
		for _, c := range cs {
			if c.method == WS {
				for _, expected := range messages[path] {
					message := c.receive()
					if message != expected {
						t.Error("expected", expected, "got", message)
					}
				}
			}
		}
	}
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

func mockWsClient(t *testing.T, url *url.URL, c *client) *websocket.Conn {
	dialer := &websocket.Dialer{
		NetDial: func(network, addr string) (net.Conn, error) { return net.Dial(network, url.Host) },
		Proxy: http.ProxyFromEnvironment,
		HandshakeTimeout: 2 * time.Second,
		Subprotocols: []string{"p1", c.message},
	}
	t.Log("dialer:", dialer)
	ws, resp, err := dialer.Dial(url.String(), nil)
	if err != nil {
		t.Fatal("dial error:", err, "resp:", resp)
	}
	return ws
}

type client struct{
	message string
	method int
	ws *websocket.Conn
	tx chan string
	rx chan bool
	res chan string
}

func (c *client) run(t *testing.T) {
	defer close(c.tx)
	defer close(c.rx)
	var message string
	for { select {
	case message = <- c.tx :
		err := c.ws.WriteMessage(
			websocket.TextMessage, []byte(message))
		if err != nil {
			t.Fatal("WriteMessage:", err)
		}
		fmt.Println("WS", message)
		c.res <- "OK"
	case <-c.rx :
		_, message, err := c.ws.ReadMessage()
		if err != nil {
			t.Fatal("ReadMessage:", err)
		}
		c.res <- string(message)
	}}
}

func (c *client) send(message string) {
	c.tx<- message
	<-c.res
}

func (c *client) receive() string {
	c.rx <- true
	message := <-c.res
	return string(message)
}
