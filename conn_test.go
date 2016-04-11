package main

import (
	"errors"
	"testing"
)

func TestConnProcessReadMessage(t *testing.T) {
	conn := newTestConnection()

	// Assert on error, do nothing
	conn.w = mockWsInteractor{err: errors.New("Message Read Error")}
	err := conn.processReadMessage()

	if err == nil {
		t.Fatal("No Error Returned")
	}

	if len(conn.send) != 0 {
		t.Fatal("Expectation: send channel length should be 0, Received:", len(conn.send))
	}

	if len(conn.channel.queue) != 0 {
		t.Fatal("Expectation: conn channel length should be 0, Received:", len(conn.channel.queue))
	}

	// On receipt of non-nil message, message is posted to queue
	conn.w = mockWsInteractor{msg: []byte("banana")}
	err = conn.processReadMessage()

	cmd := <-conn.channel.queue
	if string(cmd.text) != "banana" {
		t.Fatal("Expectation: banana, Received:", string(cmd.text))
	}

	if err != nil {
		t.Fatal("Expectation: Error should be nil, Received:", err)
	}

	// On receipt of nil message, nil message published to conn.send
	conn.w = mockWsInteractor{msg: []byte("")}
	err = conn.processReadMessage()

	if len(conn.send) != 1 {
		t.Fatal("Expectation: send channel length should be 1, Received:", len(conn.send))
	}

	if err != nil {
		t.Fatal("Expectation: Error should be nil, Received:", err)
	}
}

func newTestConnection() *connection {
	return &connection{
		control: make(chan *channel, 1),
		send:    make(chan []byte, 256),
		channel: &channel{queue: make(queue, 16), path: "/monkey"},
	}
}

type mockWsInteractor struct {
	msg []byte
	err error
}

func (mq mockWsInteractor) wsSetReadLimit() {}

func (mq mockWsInteractor) wsSetReadDeadline() {}

func (mq mockWsInteractor) wsSetPongHandler() {}

func (mq mockWsInteractor) wsClose() {}

func (mq mockWsInteractor) wsReadMessage() (messageType int, p []byte, err error) {
	return messageType, mq.msg, mq.err
}
