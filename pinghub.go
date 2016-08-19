// Package pinghub serves as a simple message bus (pubsub) over websockets.
//
//     pinghub -addr=:8081
//
// Everything is as ephemeral as can be. A message is sent to connected
// subscribers (if any) and then forgotten. A channel is forgotten when
// its last subscriber disconnects.
//
// Subscribe to a channel by opening a websocket to a valid path.
//     ws://localhost:8081/Path_must_be_valid_UTF-8
//
// Publish by sending a plain text websocket message (valid UTF-8).
//
// Publish by POSTing to the same path with a plain text body.
//     curl localhost:8081/Path_must_be_valid_UTF-8 -d "Hello"
//
// Messages are sent to all subscribers connected to the path, regardless
// of whether they were also the sender.
//
// Paths and messages must be valid UTF-8. Paths can be 1-256 characters.
// Message length should be limited but it is not.
//
// Non-websocket GET requests are served HTML with a websocket client that
// connects to the requested path.
//     http://localhost:8081/Path_must_be_valid_UTF-8
package main

const (
	pathLenMin = 1
	pathLenMax = 127
)

const (
	SUBSCRIBE   = 1 // client connected
	UNSUBSCRIBE = 2 // client disconnected
	PUBLISH     = 3 // message received
	BROADCAST   = 4 // message outbound
	REMOVE      = 5 // channel has no clients
)

type queue chan command

type command struct {
	cmd  int
	conn *connection
	path string
	text []byte
}
