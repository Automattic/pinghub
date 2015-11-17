package main

import (
	"log"
)

type channel struct {
	path string
	queue queue
	connections connections
	h *hub
}

type connections map[*connection]interface {
}

func (c *channel) run() {
	defer c.stop()
	for cmd := range c.queue {
		switch cmd.cmd {
		case SUBSCRIBE:
			log.Println("chan sub")
			c.subscribe(cmd.conn)
			log.Printf("chan conns: %v\n", c.connections)
		case UNSUBSCRIBE:
			log.Println("chan unsub")
			c.unsubscribe(cmd.conn)
			log.Printf("chan conns: %v\n", c.connections)
			log.Printf("len: %v\n", len(c.connections))
			if len(c.connections) == 0 {
				return
			}
		case PUBLISH:
			log.Println("chan pub")
			c.publish(cmd.text)
		default:
			break
		}
	}
}

func (c *channel) stop() {
	log.Println("chan stop")
	close(c.queue)
	c.h.queue <- command{cmd: REMOVE, path: c.path}
}

func (c *channel) subscribe(conn *connection) {
	c.connections[conn] = nil
}

func (c *channel) unsubscribe(conn *connection) {
	if _, ok := c.connections[conn]; ok {
		close(conn.send)
		delete(c.connections, conn)
	}
}

func (c *channel) publish(text []byte) {
	for conn := range c.connections {
		select {
		case conn.send <- text:
		default:
			c.unsubscribe(conn)
		}
	}
}
