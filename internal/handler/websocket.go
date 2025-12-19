// Package handler provides HTTP handlers for the API.
package handler

import (
	"errors"

	"github.com/labstack/echo/v4"
	"github.com/kyiku/hackz-ptera-back/internal/model"
	"github.com/kyiku/hackz-ptera-back/internal/queue"
)

// WebSocketHandler handles WebSocket connections.
type WebSocketHandler struct {
	store SessionStoreInterface
	queue *queue.WaitingQueue
}

// NewWebSocketHandler creates a new WebSocketHandler.
func NewWebSocketHandler(store SessionStoreInterface, q *queue.WaitingQueue) *WebSocketHandler {
	return &WebSocketHandler{
		store: store,
		queue: q,
	}
}

// ValidateSession validates the session for WebSocket connection.
func (h *WebSocketHandler) ValidateSession(c echo.Context) error {
	cookie, err := c.Cookie("session_id")
	if err != nil || cookie == nil {
		return errors.New("no session cookie")
	}

	_, ok := h.store.Get(cookie.Value)
	if !ok {
		return errors.New("invalid session")
	}

	return nil
}

// Connect handles the WebSocket upgrade and connection.
// Note: This is a simplified version. Real implementation would use gorilla/websocket.
func (h *WebSocketHandler) Connect(c echo.Context) error {
	// Validate session first
	if err := h.ValidateSession(c); err != nil {
		return c.JSON(401, map[string]interface{}{
			"error":   true,
			"message": "セッションが無効です",
		})
	}

	// Get session
	cookie, _ := c.Cookie("session_id")
	user, _ := h.store.Get(cookie.Value)

	// Add user to queue (simplified - real impl would handle WebSocket upgrade)
	h.queue.Add(user.ID, user.Conn)

	// Broadcast queue positions
	h.queue.BroadcastPositions()

	return nil
}

// Disconnect handles WebSocket disconnection.
func (h *WebSocketHandler) Disconnect(user *model.User) {
	if user != nil {
		h.queue.Remove(user.ID)
		h.queue.BroadcastPositions()
	}
}
