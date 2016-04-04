package main

import (
	"testing"
)

func TestSubscribe(t *testing.T) {
	h := newHub()

	if len(h.channels) != 0 {
		t.Fatal("Expectation: 0, Received:", len(h.channels))
	}

	// subscribing to a new path should
	// add a (1) channel to the hub
	h.subscribe(command{cmd: SUBSCRIBE, path: "/monkey", conn: newTestConnection()})
	if len(h.channels) != 1 {
		t.Fatal("Expectation: 1, Received:", len(h.channels))
	}

	// subscribing to the same path multiple times
	// should use same channel
	h.subscribe(command{cmd: SUBSCRIBE, path: "/monkey", conn: newTestConnection()})
	h.subscribe(command{cmd: SUBSCRIBE, path: "/monkey", conn: newTestConnection()})
	h.subscribe(command{cmd: SUBSCRIBE, path: "/monkey", conn: newTestConnection()})

	if len(h.channels) != 1 {
		t.Fatal("Expectation: 1, Received:", len(h.channels))
	}

	h.subscribe(command{cmd: SUBSCRIBE, path: "/banana", conn: newTestConnection()})
	if len(h.channels) != 2 {
		t.Fatal("Expectation: 2, Received:", len(h.channels))
	}
}

func TestPublish(t *testing.T) {
	h := newHub()
	// Publishing to a non-existant channel
	// should drop meessage
	h.publish(command{cmd: PUBLISH, path: "/monkey", text: []byte("banana 1"), conn: newTestConnection()})
	if _, ok := h.channels["/monkey"]; ok {
		t.Fatal("Expectation: Channel should not exist without a Subscriber")
	}

	// Publishing to a open channel
	// Command should be pushed onto channel queue
	h.subscribe(command{cmd: SUBSCRIBE, path: "/monkey", conn: newTestConnection()})
	h.publish(command{cmd: PUBLISH, path: "/monkey", text: []byte("banana 2"), conn: newTestConnection()})
	_, cmd := <-h.channels["/monkey"].queue, <-h.channels["/monkey"].queue
	if string(cmd.text) != "banana 2" {
		t.Fatal("Expectation: banana 2, Received:", string(cmd.text))
	}
}

func TestRemove(t *testing.T) {
	h := newHub()
	h.subscribe(command{cmd: SUBSCRIBE, path: "/monkey", conn: newTestConnection()})
	h.subscribe(command{cmd: SUBSCRIBE, path: "/banana", conn: newTestConnection()})
	h.remove(command{cmd: REMOVE, path: "/monkey", conn: newTestConnection()})

	if _, ok := h.channels["/monkey"]; ok {
		t.Fatal("ERR: Channel not removed")
	}

	if _, ok := h.channels["/banana"]; !ok {
		t.Fatal("ERR: Channel removed")
	}
}

func newTestConnection() *connection {
	return &connection{
		control: make(chan *channel, 1),
	}
}
