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
			// Create a channel if needed.
			if _, ok := h.channels[cmd.path]; !ok {
				h.channels[cmd.path] = newChannel(h, cmd.path)
				go h.channels[cmd.path].run()
			}
			// Give the connection a reference to its own channel.
			cmd.conn.control <- h.channels[cmd.path]
			h.channels[cmd.path].queue <- cmd
		case PUBLISH:
			if channel, ok := h.channels[cmd.path]; ok {
				select {
				case channel.queue <- cmd:
				default:
					h.remove(cmd.path)
				}
			}
		case REMOVE:
			h.remove(cmd.path)
		default:
			panic(fmt.Sprintf("unexpected hub cmd: %v\n", cmd))
		}
	}
}

func (h *hub) remove(path string) {
	if _, ok := h.channels[path]; ok {
		delete(h.channels, path)
	}
}
