package main

import (
	"fmt"
	"log"
	"time"

	r "gopkg.in/dancannon/gorethink.v2"
)

type channel struct {
	path        string
	queue       queue
	connections connections
	h           *hub
	session     *r.Session
}

type connections map[*connection]interface {
}

func (c *channel) run() {
	// Open a connection to rethinkdb
	var err error
	c.session, err = r.Connect(r.ConnectOpts{
		Address: "localhost:28015",
		Database: "test",
	})
	if err != nil {
		log.Fatalln(err.Error())
	}
	defer c.session.Close()

	// Subscribe to the changefeed for this channel's path
	cursor, err := r.Table("pinghub").Get(c.path).Changes().Field("new_val").Field("text").Run(c.session)
	if err != nil {
		log.Fatalln(err)
	}
	defer cursor.Close()

	// Publish 
	go func() {
		var text string
		for {
			if !cursor.Next(&text) {
				break
			}
			if text != "" {
				c.queue<- command{cmd: BROADCAST, text: []byte(text)}
			}
		}
		fmt.Println("cursor.Next loop ended")
	}()

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
		case BROADCAST:
			c.broadcast(cmd.text)
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
}

func (c *channel) unsubscribe(conn *connection) {
	if _, ok := c.connections[conn]; ok {
		close(conn.send)
		delete(c.connections, conn)
	}
}

func (c *channel) publish(text []byte) {
	if len(text) == 0 {
		return
	}
	resp, err := r.Table("pinghub").Insert(map[string]interface{}{
		"id": string(c.path),
		"text": string(text),
		"time": time.Now().UnixNano(),
	}, r.InsertOpts{
		Conflict: "replace",
	}).RunWrite(c.session)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(resp)
}

func (c *channel) broadcast(text []byte) {
	if len(text) == 0 {
		return
	}
	for conn := range c.connections {
		select {
		case conn.send <- text:
		default:
			c.unsubscribe(conn)
		}
	}
}
