package main

import (
	"fmt"
	"reflect"
	"testing"
	"time"
)

func TestHubSubscribe(t *testing.T) {
	h := newHub()

	if len(h.channels) != 0 {
		t.Fatal("Expectation: 0, Received:", len(h.channels))
	}

	// subscribing to a new path should
	// add a (1) channel to the hub
	cmd := command{cmd: SUBSCRIBE, path: "/monkey", conn: newTestConnection()}
	h.subscribe(cmd)
	if len(h.channels) != 1 {
		t.Fatal("Expectation: 1, Received:", len(h.channels))
	}

	// subscribing should give connection
	// a reference to its own channel
	subchannel := <-cmd.conn.control
	if !reflect.DeepEqual(subchannel, h.channels[cmd.path]) {
		t.Fatal(fmt.Printf("Expectation: %+v\n Received: %+v\n", h.channels[cmd.path], subchannel))
	}

	c := <-h.channels[cmd.path].queue
	if !reflect.DeepEqual(c, cmd) {
		t.Fatal(fmt.Printf("Expectation: %+v\n Received: %+v\n", cmd, c))
	}

	// subscribing to the same path multiple times
	// should use same channel
	h.subscribe(command{cmd: SUBSCRIBE, path: "/monkey", conn: newTestConnection()})
	h.subscribe(command{cmd: SUBSCRIBE, path: "/monkey", conn: newTestConnection()})
	h.subscribe(command{cmd: SUBSCRIBE, path: "/monkey", conn: newTestConnection()})

	if len(h.channels) != 1 {
		t.Fatal("Expectation: 1, Received:", len(h.channels))
	}

	// subscribing to a new path opens a new channel
	h.subscribe(command{cmd: SUBSCRIBE, path: "/banana", conn: newTestConnection()})
	if len(h.channels) != 2 {
		t.Fatal("Expectation: 2, Received:", len(h.channels))
	}
}

func TestHubPublish(t *testing.T) {
	h := newHub()
	go h.run()
	// Publishing to a non-existant channel
	// should drop message
	h.queue <- command{cmd: PUBLISH, path: "/monkey", text: []byte("banana 1"), conn: newTestConnection()}
	time.Sleep(100 * time.Millisecond)
	if _, ok := h.channels["/monkey"]; ok {
		t.Fatal("Expectation: Channel should not exist without a Subscriber")
	}

	// Publishing to a subscribed channel
	// Command should be pushed onto channel queue
	conn := newTestConnection()
	h.queue <- command{cmd: SUBSCRIBE, path: "/monkey", conn: conn}
	time.Sleep(100 * time.Millisecond)
	if _, ok := h.channels["/monkey"]; !ok {
		t.Fatal("Expectation: Channel should exist")
	}
	h.queue <- command{cmd: PUBLISH, path: "/monkey", text: []byte("banana 2"), conn: newTestConnection()}
	time.Sleep(100 * time.Millisecond)
	text := <-conn.send
	if string(text) != "banana 2" {
		t.Fatal("Expectation: banana 2, Received:", string(text))
	}
}

func TestHubRemove(t *testing.T) {
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

func TestHubRun(t *testing.T) {
	h := newHub()
	cmd := command{cmd: 99, path: "/monkey", conn: newTestConnection()}
	h.queue <- cmd

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("ERR: panic did not occur with invalid command")
		}
	}()

	h.run()
}
