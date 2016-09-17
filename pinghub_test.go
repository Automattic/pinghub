package main

import (
	"flag"
	"fmt"
	"github.com/gorilla/websocket"
	"io/ioutil"
	"log"
	"math/rand"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"reflect"
	"strings"
	"sync"
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
	rnd    *rand.Rand
	seed   *int64 = flag.Int64("seed", time.Now().UnixNano(), "Seed for RNG used by fuzzer (default: time in nanoseconds)")
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
	testClientsN(t, 1, "/testpath1", 1)
	testClientsN(t, 1, randomPath(), 1)
}

func TestClients10(t *testing.T) {
	testClientsN(t, 10, "/testpath10", 2)
	testClientsN(t, 10, randomPath(), 2)
}

func TestClients100(t *testing.T) {
	testClientsN(t, 100, "/testpath100", 37)
	testClientsN(t, 100, randomPath(), 37)
}

/*
func TestClients1000(t *testing.T) {
	testClientsN(t, 1000, "/testpath1000", 241)
	testClientsN(t, 1000, randomPath(), 241)
}
*/

func randomPath() string {
	u, _ := url.Parse(server.URL)
	u.Path = "/" + quickValue("", rnd).(string)
	for {
		if len(u.EscapedPath()) < 120 {
			break
		}
		// shorten EscapedPath by removing a rune
		u.Path = string([]rune(u.Path)[1:])
	}
	return u.Path
}

func testClientsN(t *testing.T, numClients int, path string, numPaths int) {
	t.Log("Testing", numClients, "clients on path", path)
	hub := mockHub()
	clients := []*client{}
	method := WS
	lastWS := map[string]int{}
	for i := 0; i < numClients; i++ {
		if i > numPaths {
			method = quickValue(method, rnd).(bool)
		}
		if i == numClients-1 {
			method = WS
		}
		message := fmt.Sprintf("%v_%v", i, quickValue("", rnd).(string))
		newClient := mockClient(method, TESTORIGIN)
		newClient.path = fmt.Sprintf("%v_%v", path, i%numPaths)
		clients = append(clients, newClient)
		c := clients[i]
		u, _ := url.Parse(server.URL)
		u.Path = c.path
		switch c.method {

		case WS:
			u.Scheme = "ws"
			ws, err := mockWs(t, u, c)
			if err != nil {
				t.Fatal("dial error:", err)
			}
			c.ws = ws
			defer c.ws.Close()
			hub.subscribe(c.path, c)
			go c.reader()
			if i%5 == 0 {
				c.sendSync(t, message)
				hub.send(c.path, message)
			}
			lastWS[c.path] = i

		case POST:
			// Wait until the last WS client receives
			clients[lastWS[c.path]].waiting = true
			resp := post(t, u, message)
			if resp.Status != "200 OK" || string(responseBody(t, resp)) != "OK\n" {
				t.Fatal("POST response not 200 OK:", resp)
			}
			hub.send(c.path, message)
			<-clients[lastWS[c.path]].res
			clients[lastWS[c.path]].waiting = false
		}
	}

	// Give the server some time to transact with all clients.
	time.Sleep(time.Second)

	t.Log("TestClients: clients receive messages in order")
	for i := range clients {
		c := clients[i]
		if c.method == WS {
			expected := strings.Join(hub.receiveAll(c), "  ")
			got := strings.Join(c.readAll(), "  ")
			c.ws.Close()
			if !strings.HasSuffix(got, expected) {
				t.Fatal("client", i, "path", c.path, "expected", expected, "got", got)
			}
		}
	}
}

func quickValue(x interface{}, r *rand.Rand) interface{} {
	t := reflect.TypeOf(x)
	value, ok := quick.Value(t, r)
	if !ok {
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
	if message == "" {
		return
	}
	for c := range h.m[path] {
		h.m[path][c] <- message
	}
}

func (h *fakeHub) receiveAll(client *client) (messages []string) {
	for {
		select {
		case m := <-h.m[client.path][client]:
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
	sync.Mutex

	method  bool
	waiting bool
	ws      *websocket.Conn
	res     chan string
	rec     []string
	origin  string
	path    string
}

func mockClient(method bool, origin string) *client {
	if method == POST {
		return &client{method: method, origin: origin}
	}
	return &client{
		method: method,
		res:    make(chan string),
		rec:    []string{},
		origin: origin,
	}
}

func (c *client) reader() {
	for {
		_, message, err := c.ws.ReadMessage()
		if err != nil {
			return
		}
		c.Lock()
		c.rec = append(c.rec, string(message))
		c.Unlock()
		if c.waiting {
			c.res <- string(message)
		}
	}
}

// Send a message and block until echo is received
func (c *client) sendSync(t *testing.T, message string) {
	if message != "" {
		c.waiting = true
	}
	err := c.ws.WriteMessage(
		websocket.TextMessage, []byte(message))
	if err != nil {
		t.Fatal("WriteMessage:", err)
	}
	if message != "" {
		ok := false
		var m string
	resloop:
		for {
			select {
			case m, ok = <-c.res:
				if m == message {
					c.waiting = false
					break resloop
				}
			case <-time.After(time.Second * 2):
				ok = false
			}
			if !ok {
				t.Fatal("Failed waiting for echo.")
			}
		}
	}
}

func (c *client) readAll() []string {
	c.Lock()
	all := c.rec
	c.Unlock()
	return all
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
