package main

import (
	"fmt"
	"flag"
	"github.com/gorilla/websocket"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"math/rand"
	"reflect"
	"strings"
	"testing"
	"testing/quick"
	"time"
)

const (
	WS         = true
	POST       = false
	TESTORIGIN = "https://example.com"
	BADORIGIN1 = "https://example.net"
	BADORIGIN2 = "http://example.com"
)

var (
	rnd *rand.Rand
	seed *int64 = flag.Int64("seed", time.Now().UnixNano(), "Seed for RNG used by fuzzer (default: time in nanoseconds)")
	server *httptest.Server
)

func TestMain(m *testing.M) {
	flag.Parse()
	rnd = rand.New(rand.NewSource(*seed))
	fmt.Println("TestMain: rand seed:", *seed, "(command line flag '-seed=N')")

	server = httptest.NewServer(newHandler(TESTORIGIN))
	defer server.Close()
	u, err := url.Parse(server.URL)
	if err != nil {
		log.Fatal("Server URL parse error:", err)
	}
	fmt.Println("TestMain: server addr:", u)

	os.Exit(m.Run())
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

func TestBadOrigin1(t *testing.T) {
	t.Log("TestOrigin: Origin host mismatch returns 403.")
	client := mockClient(WS, BADORIGIN1)
	u, _ := url.Parse(server.URL)
	u.Path = "/testpath"
	u.Scheme = "ws"
	_, err := mockWs(t, u, client)
	if err == nil {
		t.Fatal("TestOrigin: server failed to return 403 for a bad origin.")
	}
}

func TestBadOrigin2(t *testing.T) {
	t.Log("TestOrigin: Origin scheme mismatch returns 403.")
	client := mockClient(WS, BADORIGIN2)
	u, _ := url.Parse(server.URL)
	u.Path = "/testpath"
	u.Scheme = "ws"
	_, err := mockWs(t, u, client)
	if err == nil {
		t.Fatal("TestOrigin: server failed to return 403 for a bad origin.")
	}
}

func TestClients1(t *testing.T) {
	testClientsN(t, 1, "/testpath1")
	testClientsN(t, 1, randomPath())
}

func TestClients10(t *testing.T) {
	testClientsN(t, 10, "/testpath10")
	testClientsN(t, 10, randomPath())
}

func TestClients100(t *testing.T) {
	testClientsN(t, 100, "/testpath100")
	testClientsN(t, 100, randomPath())
}

/*
func TestClients1000(t *testing.T) {
	testClientsN(t, 1000, "/testpath1000")
	testClientsN(t, 1000, randomPath())
}
*/

func randomPath() string {
	return "/" + quickValue("", rnd).(string)
}

func testClientsN(t *testing.T, numClients int, path string) {
	t.Log("Testing", numClients, "clients on path", path)
	hub := mockHub()
	clients := []*client{}
	for i := 0; i < numClients; i++ {
		method := quickValue(true, rnd).(bool)
		message := quickValue("", rnd).(string)
		newClient := mockClient(method, TESTORIGIN)
		clients = append(clients, newClient)
		c := clients[i]
		u, _ := url.Parse(server.URL)
		u.Path = path
		switch c.method {

		case WS:
			u.Scheme = "ws"
			ws, err := mockWs(t, u, c)
			if err != nil {
				t.Fatal("dial error:", err)
			}
			c.ws = ws
			defer c.ws.Close()
			hub.subscribe(path, c)
			go c.reader()
			c.sendSync(t, message)
			hub.send(path, message)

		case POST:
			resp := post(t, u, message)
			if resp.Status != "200 OK" || string(responseBody(t, resp)) != "OK\n" {
				t.Fatal("POST response not 200 OK:", resp)
			}
			hub.send(path, message)
		}
	}

	t.Log("TestClients: clients receive messages in order")
	// Give the server some time to transact with all clients.
	time.Sleep(50 * time.Millisecond)
	for i := range clients {
		c := clients[i]
		if c.method == WS {
			expected := strings.Join(hub.receiveAll(path, c), "")
			got := c.readAll()
			if expected != got {
				t.Fatal("expected", expected, "got", got)
			}
		}
	}
}

func quickValue(x interface{}, r *rand.Rand) interface{} {
	t := reflect.TypeOf(x)
	value, ok := quick.Value(t, r)
	if ! ok {
		panic("Failed to create a quick value: " + t.Name())
	}
	return value.Interface()
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
	method  bool
	waiting bool
	ws      *websocket.Conn
	res     chan struct{}
	rec     []string
	origin  string
}

func mockClient(method bool, origin string) *client {
	if method == POST {
		return &client{method, false, nil, nil, nil, origin}
	}
	return &client{
		method,
		false,
		nil,
		make(chan struct{}),
		[]string{},
		origin,
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

func mockWs(t *testing.T, u *url.URL, c *client) (*websocket.Conn, error) {
	dialer := &websocket.Dialer{
		NetDial: func(network, addr string) (net.Conn, error) {
			d := net.Dialer{
				Timeout: 3 * time.Second,
			}
			return d.Dial(network, u.Host)
		},
		Proxy:            http.ProxyFromEnvironment,
		HandshakeTimeout: 3 * time.Second,
	}
	requestHeader := http.Header{}
	if c.origin != "" {
		requestHeader.Set("Origin", c.origin)
	}
	ws, _, err := dialer.Dial(u.String(), requestHeader)
	return ws, err
}
