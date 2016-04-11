package main

import (
	"github.com/gorilla/websocket"
	"time"
)

const (
	// Time allowed to write a message to the peer.
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer.
	pongWait = 30 * time.Second

	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10

	// Maximum message size allowed from peer.
	maxMessageSize = 512
)

type connection struct {
	control chan *channel
	channel *channel
	send    chan []byte
	ws      *websocket.Conn
	h       *hub
	path    string
	w       websocketManager
}

func newConnection(ws *websocket.Conn, h *hub, path string) *connection {
	return &connection{
		control: make(chan *channel, 1),
		send:    make(chan []byte, 256),
		ws:      ws,
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
		c.ws.Close()
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
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.ws.Close()
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
		case <-ticker.C:
			if err := c.write(websocket.PingMessage, []byte{}); err != nil {
				return
			}
		}
	}
}

func (c *connection) write(mt int, payload []byte) error {
	c.ws.SetWriteDeadline(time.Now().Add(writeWait))
	return c.ws.WriteMessage(mt, payload)
}

type websocketManager interface {
	wsSetReadLimit()
	wsSetReadDeadline()
	wsSetPongHandler()
	wsReadMessage() (int, []byte, error)
	wsClose()
}

type websocketInteractor struct {
	ws *websocket.Conn
}

func (w websocketInteractor) wsSetReadLimit() {
	w.ws.SetReadLimit(maxMessageSize)
}

func (w websocketInteractor) wsSetReadDeadline() {
	w.ws.SetReadDeadline(time.Now().Add(pongWait))
}

func (w websocketInteractor) wsSetPongHandler() {
	w.ws.SetPongHandler(func(s string) error { w.wsSetReadDeadline(); return nil })
}

func (w websocketInteractor) wsClose() {
	w.ws.Close()
}

func (w websocketInteractor) wsReadMessage() (messageType int, p []byte, err error) {
	return w.ws.ReadMessage()
}
