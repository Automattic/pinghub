package main

import (
	"errors"
	"testing"
	"time"
)

var testWrite []byte
var testInt int

func TestConnReadMessage(t *testing.T) {
	conn := newTestConnection()

	// Assert on error, do nothing
	conn.w = mockWsInteractor{err: errors.New("Message Read Error")}
	err := conn.readMessage()

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
	err = conn.readMessage()

	cmd := <-conn.channel.queue
	if string(cmd.text) != "banana" {
		t.Fatal("Expectation: banana, Received:", string(cmd.text))
	}

	if err != nil {
		t.Fatal("Expectation: Error should be nil, Received:", err)
	}

	// On receipt of nil message, nil message published to conn.send
	conn.w = mockWsInteractor{msg: []byte("")}
	err = conn.readMessage()

	if len(conn.send) != 1 {
		t.Fatal("Expectation: send channel length should be 1, Received:", len(conn.send))
	}

	if err != nil {
		t.Fatal("Expectation: Error should be nil, Received:", err)
	}
}

func TestConnWriter(t *testing.T) {
	conn := newTestConnection()
	conn.w = mockWsInteractor{}

	go conn.writer(2 * time.Second)
	conn.send <- []byte("bananas")

	// On receipt of valid message, message written
	// with type websocket.TextMessage
	time.Sleep(5)
	if string(testWrite) != "bananas" {
		t.Fatal("Expectation: bananas, Received:", string(testWrite))
	}

	if testInt != 1 {
		t.Fatal("Expectation: 1, Received:", testInt)
	}

	// On timed intervals, ping with nil message
	// and type websocket.PingMessage
	time.Sleep(3 * time.Second)
	if string(testWrite) != "" {
		t.Fatal("Expectation: nil, Received:", string(testWrite))
	}
	if testInt != 9 {
		t.Fatal("Expectation: 9, Received:", testInt)
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

func (mq mockWsInteractor) wsSetWriteDeadline() {}

func (mq mockWsInteractor) wsReadMessage() (messageType int, p []byte, err error) {
	return messageType, mq.msg, mq.err
}

func (mq mockWsInteractor) wsWriteMessage(messageType int, payload []byte) (err error) {
	testInt = messageType
	testWrite = payload
	return mq.err
}
