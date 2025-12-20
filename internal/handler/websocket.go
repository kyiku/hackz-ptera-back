// Package handler provides HTTP handlers for the API.
package handler

import (
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v4"
	"github.com/kyiku/hackz-ptera-back/internal/model"
	"github.com/kyiku/hackz-ptera-back/internal/queue"
	ws "github.com/kyiku/hackz-ptera-back/internal/websocket"
)

// upgrader is the WebSocket upgrader with default settings.
var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		// Allow all origins for now (configure properly in production)
		return true
	},
}

// WebSocketConn wraps gorilla/websocket.Conn to implement model.WebSocketConn.
type WebSocketConn struct {
	conn *websocket.Conn
	mu   sync.Mutex
}

// WriteMessage sends a message with the given type.
func (c *WebSocketConn) WriteMessage(messageType int, data []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.conn.WriteMessage(messageType, data)
}

// WriteJSON sends a JSON message.
func (c *WebSocketConn) WriteJSON(v interface{}) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.conn.WriteJSON(v)
}

// Close closes the WebSocket connection.
func (c *WebSocketConn) Close() error {
	return c.conn.Close()
}

// ReadMessage reads a message from the connection.
func (c *WebSocketConn) ReadMessage() (messageType int, p []byte, err error) {
	return c.conn.ReadMessage()
}

// SessionStoreForWS defines the session store interface for WebSocket handler.
type SessionStoreForWS interface {
	Create() (*model.User, string)
	Get(sessionID string) (*model.User, bool)
}

// WebSocketHandler handles WebSocket connections.
type WebSocketHandler struct {
	store SessionStoreForWS
	queue *queue.WaitingQueue
}

// NewWebSocketHandler creates a new WebSocketHandler.
func NewWebSocketHandler(store SessionStoreForWS, q *queue.WaitingQueue) *WebSocketHandler {
	return &WebSocketHandler{
		store: store,
		queue: q,
	}
}

// ValidateSession validates the session for WebSocket connection.
func (h *WebSocketHandler) ValidateSession(c echo.Context) error {
	cookie, err := c.Cookie("session_id")
	if err != nil || cookie == nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "no session cookie")
	}

	_, ok := h.store.Get(cookie.Value)
	if !ok {
		return echo.NewHTTPError(http.StatusUnauthorized, "invalid session")
	}

	return nil
}

// Connect handles the WebSocket upgrade and connection.
func (h *WebSocketHandler) Connect(c echo.Context) error {
	// Get or create session
	var user *model.User
	var sessionID string

	cookie, err := c.Cookie("session_id")
	if err != nil || cookie == nil {
		// Create new session
		user, sessionID = h.store.Create()
		c.SetCookie(&http.Cookie{
			Name:     "session_id",
			Value:    sessionID,
			Path:     "/",
			HttpOnly: true,
		})
	} else {
		// Get existing session
		var ok bool
		user, ok = h.store.Get(cookie.Value)
		if !ok {
			// Session not found, create new one
			user, sessionID = h.store.Create()
			c.SetCookie(&http.Cookie{
				Name:     "session_id",
				Value:    sessionID,
				Path:     "/",
				HttpOnly: true,
			})
		}
	}

	// Upgrade to WebSocket
	wsConn, err := upgrader.Upgrade(c.Response(), c.Request(), nil)
	if err != nil {
		log.Printf("WebSocket upgrade failed: %v", err)
		return err
	}

	// Wrap the connection
	conn := &WebSocketConn{conn: wsConn}
	user.Conn = conn

	// Add user to queue
	h.queue.Add(user.ID, conn)
	log.Printf("User %s connected, queue position: %d", user.ID, h.queue.Len())

	// Broadcast positions to all users
	h.queue.BroadcastPositions()

	// Send welcome message
	_ = conn.WriteJSON(map[string]interface{}{
		"type":    "connected",
		"message": "待機列に参加しました",
		"user_id": user.ID,
	})

	// Handle messages in a goroutine
	go h.handleMessages(user, conn)

	return nil
}

// handleMessages handles incoming WebSocket messages.
func (h *WebSocketHandler) handleMessages(user *model.User, conn *WebSocketConn) {
	pingHandler := ws.NewPingHandler(conn)

	// Set read deadline for ping/pong
	_ = conn.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	conn.conn.SetPongHandler(func(string) error {
		_ = conn.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	defer func() {
		// Clean up on disconnect
		h.queue.Remove(user.ID)
		h.queue.BroadcastPositions()
		conn.Close()
		log.Printf("User %s disconnected", user.ID)
	}()

	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error: %v", err)
			}
			break
		}

		// Handle ping messages
		if pingHandler.Handle(message) {
			// Reset read deadline on ping
			_ = conn.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
			continue
		}

		// Handle other message types here if needed
		log.Printf("Received message from %s: %s", user.ID, string(message))
	}
}

// PromoteFirstUser promotes the first user in the queue to the next stage.
// This is called when the queue wait time is complete.
func (h *WebSocketHandler) PromoteFirstUser() *model.User {
	queueUser := h.queue.PopFront()
	if queueUser == nil {
		return nil
	}

	// Get the full user from store by finding it
	// Note: In a real implementation, you'd want to store user reference in QueueUser
	if queueUser.Conn != nil {
		_ = queueUser.Conn.WriteJSON(map[string]interface{}{
			"type":    "stage_change",
			"status":  model.StatusStage1Dino,
			"message": "あなたの番です！Dino Runを開始してください",
		})
	}

	// Broadcast updated positions to remaining users
	h.queue.BroadcastPositions()

	return nil
}
