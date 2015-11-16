package main

import (
	"github.com/gorilla/websocket"
)

type connection struct {
	control chan *channel
	channel *channel
	send    chan []byte
	ws      *websocket.Conn
	h       *hub
	path    string
}

func newConnection(ws *websocket.Conn, h *hub, path string) *connection {
	return &connection{
		control: make(chan *channel, 1),
		send: make(chan []byte, 256),
		ws:   ws,
		h:    h,
		path: path,
	}
}

func (c *connection) run() {
	c.h.queue <- command{cmd: SUBSCRIBE, conn: c, path: c.path}
	c.channel = <- c.control
	close(c.control)
	defer func() { c.channel.queue <- command{cmd: UNSUBSCRIBE, conn: c, path: c.path} }()
	go c.writer()
	c.reader()
}

func (c *connection) reader() {
	for {
		_, message, err := c.ws.ReadMessage()
		if err != nil {
			break
		}
		c.channel.queue <- command{cmd: PUBLISH, path: c.path, text: message}
	}
	c.ws.Close()
}

func (c *connection) writer() {
	for message := range c.send {
		err := c.ws.WriteMessage(websocket.TextMessage, message)
		if err != nil {
			break
		}
	}
	c.ws.Close()
}
