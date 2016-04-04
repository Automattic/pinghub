package main

import (
	"testing"
)

func TestSubscribe(t *testing.T) {
	h := newTestHub()

	if len(h.channels) != 0 {
		t.Fatal("Expectation: 0, Received:", len(h.channels))
	}

	// subscribing to a new path should
	// add a (1) channel to the hub
	h.subscribe(command{cmd: SUBSCRIBE, path: "/monkey"})
	if len(h.channels) != 1 {
		t.Fatal("Expectation: 1, Received:", len(h.channels))
	}

	// subscribing to the same path multiple times
	// should use same channel
	h.subscribe(command{cmd: SUBSCRIBE, path: "/monkey"})
	h.subscribe(command{cmd: SUBSCRIBE, path: "/monkey"})
	h.subscribe(command{cmd: SUBSCRIBE, path: "/monkey"})

	if len(h.channels) != 1 {
		t.Fatal("Expectation: 1, Received:", len(h.channels))
	}

	h.subscribe(command{cmd: SUBSCRIBE, path: "/banana"})
	if len(h.channels) != 2 {
		t.Fatal("Expectation: 2, Received:", len(h.channels))
	}
}

func TestPublish(t *testing.T) {
	h := newTestHub()
	// Publishing to a non-existant channel
	// should drop meessage
	h.publish(command{cmd: PUBLISH, path: "/monkey", text: []byte("banana 1")})
	if _, ok := h.channels["/monkey"]; ok {
		t.Fatal("Expectation: Channel should not exist without a Subscriber")
	}

	// Publishing to a open channel
	// Command should be pushed onto channel queue
	h.subscribe(command{cmd: SUBSCRIBE, path: "/monkey"})
	h.publish(command{cmd: PUBLISH, path: "/monkey", text: []byte("banana 2")})
	_, cmd := <-h.channels["/monkey"].queue, <-h.channels["/monkey"].queue
	if string(cmd.text) != "banana 2" {
		t.Fatal("Expectation: banana 2, Received:", string(cmd.text))
	}
}

func TestRemove(t *testing.T) {
	h := newTestHub()
	h.subscribe(command{cmd: SUBSCRIBE, path: "/monkey"})
	h.subscribe(command{cmd: SUBSCRIBE, path: "/banana"})
	h.remove(command{cmd: REMOVE, path: "/monkey"})

	if _, ok := h.channels["/monkey"]; ok {
		t.Fatal("ERR: Channel not removed")
	}

	if _, ok := h.channels["/banana"]; !ok {
		t.Fatal("ERR: Channel removed")
	}
}

func newTestHub() *hub {
	return &hub{
		queue:             make(queue, 16),
		channels:          make(channels),
		connectionManager: stubConnectionManager{},
	}
}

type stubConnectionManager struct{}

func (scm stubConnectionManager) hubSubscribe(cmd command, h *hub) {}
