package main

import (
	"errors"
	"testing"
	"time"
)

func TestConnWriterErr(t *testing.T) {
	conn := newTestConnection()
	conn.w = mockWsInteractor{err: errors.New("Message Read Error")}

	conn.reader()

	if len(conn.send) != 0 {
		t.Fatal("Expectation: send channel length should be > 0, Received:", len(conn.send))
	}

	if len(conn.channel.queue) != 0 {
		t.Fatal("Expectation: conn channel length should be > 0, Received:", len(conn.channel.queue))
	}
}

func TestConnWriterMessage(t *testing.T) {
	conn := newTestConnection()
	conn.w = mockWsInteractor{msg: []byte("banana")}

	defer func() {
		if r := recover(); r != nil {
			cmd := <-conn.channel.queue
			if string(cmd.text) != "banana" {
				t.Fatal("Expectation: banana, Received:", string(cmd.text))
			}
		}
	}()
	go conn.reader()
	panic("kill infinite loop")
}

func TestConnWriterNilMessage(t *testing.T) {
	conn := newTestConnection()
	conn.w = mockWsInteractor{msg: []byte("")}

	if len(conn.send) != 0 {
		t.Fatal("Expectation: 0, Received:", len(conn.send))
	}
	defer func() {
		if r := recover(); r != nil {
			// wait to eliminate possibility of race condition
			time.Sleep(time.Nanosecond * 50000000)
			if len(conn.send) == 0 {
				t.Fatal("Expectation: send channel length should be > 0, Received:", len(conn.send))
			}
		}
	}()
	go conn.reader()
	panic("kill infinite loop")
}

// func newTestConnectionWS() *connection {
// 	conn := newTestConnection()
// 	conn.channel = &channel{queue: make(queue, 16), path: "/monkey"}
// 	return conn
// }

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
