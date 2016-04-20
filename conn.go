package main

import (
	"github.com/gorilla/websocket"
)

type connection struct {
	control chan *channel
	channel *channel
	send    chan []byte
	h       *hub
	path    string
	w       websocketManager
}

func newConnection(ws *websocket.Conn, h *hub, path string) *connection {
	return &connection{
		control: make(chan *channel, 1),
		send:    make(chan []byte, 256),
		h:       h,
		path:    path,
		w:       websocketInteractor{ws: ws},
	}
}

func (c *connection) run() {
	c.h.queue <- command{cmd: SUBSCRIBE, conn: c, path: c.path}
	c.channel = <-c.control
	close(c.control)
	incr("websockets", 1)
	defer func() {
		decr("websockets", 1)
		c.channel.queue <- command{cmd: UNSUBSCRIBE, conn: c, path: c.path}
	}()
	go c.writer()
	c.reader()
}

func (c *connection) reader() {
	c.w.wsSetReadLimit()
	c.w.wsSetReadDeadline()
	c.w.wsSetPongHandler()

	defer func() {
		c.w.wsClose()
	}()

	for {
		err := c.readMessage()
		if err != nil {
			break
		}
	}
}

func (c *connection) readMessage() (err error) {
	_, message, err := c.w.wsReadMessage()
	if err != nil {
		return err
	}
	// empty message: echo only, no broadcast
	if len(message) == 0 {
		c.send <- []byte{}
		return
	}
	c.channel.queue <- command{cmd: PUBLISH, path: c.path, text: message}
	mark("websocketmsgs", 1)
	return
}

func (c *connection) writer() {
	subscriber := c.h.ticker.subscribe()
	defer func() {
		c.h.ticker.unsubscribe(subscriber)
		c.w.wsClose()
	}()

	for {
		select {
		case message, ok := <-c.send:
			if !ok {
				c.write(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.write(websocket.TextMessage, message); err != nil {
				return
			}

			mark("sends", 1)
		case <-subscriber.tick:
			if err := c.write(websocket.PingMessage, []byte{}); err != nil {
				return
			}
		}
	}
}

func (c *connection) write(mt int, payload []byte) error {
	c.w.wsSetWriteDeadline()
	return c.w.wsWriteMessage(mt, payload)
}
