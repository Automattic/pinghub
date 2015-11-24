package main

import (
	"fmt"
)

type hub struct {
	queue    queue
	channels channels
}

type channels map[string]*channel

func newHub() *hub {
	return &hub{
		queue:    make(queue, 16),
		channels: make(channels),
	}
}

func newChannel(h *hub, path string) *channel {
	return &channel{
		queue:       make(queue, 16),
		connections: make(connections),
		h:           h,
		path:        path,
	}
}

func (h *hub) run() {
	for cmd := range h.queue {
		// Forward cmds to their path's channel queues.
		switch cmd.cmd {
		case SUBSCRIBE:
			h.subscribe(cmd)
		case PUBLISH:
			h.publish(cmd)
		case REMOVE:
			h.remove(cmd)
		default:
			panic(fmt.Sprintf("unexpected hub cmd: %v\n", cmd))
		}
	}
}

func (h *hub) subscribe(cmd command) {
	// Create a channel if needed.
	if _, ok := h.channels[cmd.path]; !ok {
		h.channels[cmd.path] = newChannel(h, cmd.path)
		go h.channels[cmd.path].run()
		incr("hub.subscribe.newChannel", 1)
		incr("hub.channels", 1)
	}
	// Give the connection a reference to its own channel.
	cmd.conn.control <- h.channels[cmd.path]
	h.channels[cmd.path].queue <- cmd
	incr("hub.subscribe", 1)
}

func (h *hub) publish(cmd command) {
	if channel, ok := h.channels[cmd.path]; ok {
		select {
		case channel.queue <- cmd:
			incr("hub.publish", 1)
		default:
			// Tried publishing to a closing channel.
			h.remove(cmd)
		}
	}
}

func (h *hub) remove(cmd command) {
	if _, ok := h.channels[cmd.path]; ok {
		delete(h.channels, cmd.path)
		incr("hub.remove", 1)
		decr("hub.channels", 1)
	}
}
