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

type websocketManager interface {
	wsSetReadLimit()
	wsSetReadDeadline()
	wsSetPongHandler()
	wsReadMessage() (int, []byte, error)
	wsSetWriteDeadline()
	wsWriteMessage(int, []byte) error
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

func (w websocketInteractor) wsSetWriteDeadline() {
	w.ws.SetWriteDeadline(time.Now().Add(writeWait))
}

func (w websocketInteractor) wsWriteMessage(messageType int, payload []byte) error {
	return w.ws.WriteMessage(messageType, payload)
}
