package main

import (
	"testing"
)

func TestChannelSubscribe(t *testing.T) {
	c := newChannel(newHub(), "/monkey")

	// Assert no channels exist
	if len(c.connections) != 0 {
		t.Fatal("Error in test enviroment, Expectation: 0, Received:", len(c.connections))
	}

	c.subscribe(newTestConnection())
	if len(c.connections) != 1 {
		t.Fatal("Expectation: 1, Received:", len(c.connections))
	}
}

func TestChannelPublish(t *testing.T) {
	c := newChannel(newHub(), "/monkey")
	conn := newTestConnection()

	// Assert no channels exist
	if len(c.connections) != 0 {
		t.Fatal("Error in test enviroment, Expectation: 0, Received:", len(c.connections))
	}

	// Subscribe and Publish text to Channel
	c.subscribe(conn)
	c.publish([]byte("monkey"))
	text := <-conn.send
	if string(text) != "monkey" {
		t.Fatal("Expectation: published text should be 'monkey', Received:", string(text))
	}

	// Publish should occur on all of the channel's connections
	conn2 := newTestConnection()
	c.subscribe(conn2)
	c.publish([]byte("banana"))

	text1, text2 := <-conn.send, <-conn2.send
	if string(text1) != "banana" || string(text2) != "banana" {
		t.Fatal("Expectation: published text for connections should be 'banana', Received:", string(text1), string(text1))
	}

	//Empty string should not publish
	c.publish([]byte(""))
	if len(conn.send) != 0 {
		t.Fatal("Expectation: 0 - nil/empty string should not publish, Received:", len(conn.send))
	}

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
