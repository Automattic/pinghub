package main

import (
	"testing"
)

func TestChannelSubscribe(t *testing.T) {
	c := newChannel(newHub(), "/monkey")

	// Assert no connections exist
	if len(c.connections) != 0 {
		t.Fatal("Error in test enviroment, Expectation: 0, Received:", len(c.connections))
	}

	c.subscribe(newTestConnection())
	if len(c.connections) != 1 {
		t.Fatal("Expectation: 1, Received:", len(c.connections))
	}
}

func TestChannelPublish(t *testing.T) {
	h := newHub()
	go h.run()
	c := newChannel(h, "/monkey")
	go c.run()
	conn := newTestConnection()

	// Assert no connections exist
	if len(c.connections) != 0 {
		t.Fatal("Error in test environment, Expectation: 0, Received:", len(c.connections))
	}

	// Subscribe and Publish text to Channel
	c.h.queue <- command{cmd: SUBSCRIBE, conn: conn, path: c.path}
	c.queue<- command{cmd: PUBLISH, text: []byte("monkey")}
	text := <-conn.send
	if string(text) != "monkey" {
		t.Fatal("Expectation: published text should be 'monkey', Received:", string(text))
	}

	// Publish should occur on all of the channel's connections, unless message empty
	conn2 := newTestConnection()
	c.h.queue <- command{cmd: SUBSCRIBE, conn: conn2, path: c.path}
	c.queue<- command{cmd: PUBLISH, text: []byte("")}
	c.queue<- command{cmd: PUBLISH, text: []byte("banana")}

	text1, text2 := <-conn.send, <-conn2.send
	if string(text1) != "banana" || string(text2) != "banana" {
		t.Fatal("Expectation: published text for connections should be 'banana', Received:", string(text1), string(text1))
	}

	c.queue<- command{cmd: UNSUBSCRIBE, conn: conn}
	c.queue<- command{cmd: UNSUBSCRIBE, conn: conn2}
}

func TestChannelUnsubscribe(t *testing.T) {
	c := newChannel(newHub(), "/monkey")
	conn := newTestConnection()
	c.subscribe(conn)

	c.unsubscribe(conn)
	if len(c.connections) != 0 {
		t.Fatal("Expectation: 0, Received:", len(c.connections))
	}

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("ERR: channel conn.send not closed")
		}
	}()

	conn.send <- []byte("")
}

func TestChannelStop(t *testing.T) {
	c := newChannel(newHub(), "/monkey")
	c.stop()
	cmd := <-c.h.queue

	if cmd.cmd != REMOVE {
		t.Fatal("Expectation: cmd 4, Received:", cmd.cmd)
	}
}
