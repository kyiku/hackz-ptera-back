// Package websocket provides WebSocket message handling utilities.
package websocket

import (
	"encoding/json"

	"github.com/kyiku/hackz-ptera-back/internal/model"
)

// PingHandler handles ping/pong messages for WebSocket connections.
type PingHandler struct {
	conn model.WebSocketConn
}

// NewPingHandler creates a new PingHandler.
func NewPingHandler(conn model.WebSocketConn) *PingHandler {
	return &PingHandler{
		conn: conn,
	}
}

// Handle processes a message and returns true if it was a ping message.
func (h *PingHandler) Handle(message []byte) bool {
	var msg map[string]interface{}
	if err := json.Unmarshal(message, &msg); err != nil {
		return false
	}

	msgType, ok := msg["type"].(string)
	if !ok || msgType != "ping" {
		return false
	}

	// Send pong response
	_ = h.conn.WriteJSON(map[string]interface{}{
		"type": "pong",
	})

	return true
}

// IsPingMessage checks if a message is a ping message without processing it.
func IsPingMessage(message []byte) bool {
	var msg map[string]interface{}
	if err := json.Unmarshal(message, &msg); err != nil {
		return false
	}

	msgType, ok := msg["type"].(string)
	return ok && msgType == "ping"
}
