package main

type channel struct {
	path        string
	queue       queue
	connections connections
	h           *hub
}

type connections map[*connection]interface {
}

func (c *channel) run() {
	incr("channels", 1)
	defer c.stop()
	for cmd := range c.queue {
		switch cmd.cmd {
		case SUBSCRIBE:
			c.subscribe(cmd.conn)
		case UNSUBSCRIBE:
			c.unsubscribe(cmd.conn)
			if len(c.connections) == 0 {
				return
			}
		case PUBLISH:
			c.publish(cmd.text)
		default:
			break
		}
	}
}

func (c *channel) stop() {
	close(c.queue)
	c.h.queue <- command{cmd: REMOVE, path: c.path}
	decr("channels", 1)
}

func (c *channel) subscribe(conn *connection) {
	c.connections[conn] = nil
	incr("channel.subscribe", 1)
}

func (c *channel) unsubscribe(conn *connection) {
	if _, ok := c.connections[conn]; ok {
		close(conn.send)
		delete(c.connections, conn)
		incr("channel.unsubscribe", 1)
	}
}

func (c *channel) publish(text []byte) {
	for conn := range c.connections {
		select {
		case conn.send <- text:
			incr("channel.publish.send", 1)
		default:
			c.unsubscribe(conn)
		}
	}
	incr("channel.publish", 1)
}
