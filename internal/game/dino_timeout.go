// Package game provides game-related functionality like timeouts.
package game

import (
	"sync"
	"time"

	"github.com/kyiku/hackz-ptera-back/internal/model"
)

// QueueInterface defines the interface for the waiting queue.
type QueueInterface interface {
	Add(userID string, conn model.WebSocketConn)
}

// DinoTimeout manages the Dino Run game timeout.
type DinoTimeout struct {
	mu       sync.Mutex
	user     *model.User
	timeout  time.Duration
	timer    *time.Timer
	running  bool
	canceled bool
	queue    QueueInterface
}

// NewDinoTimeout creates a new DinoTimeout for a user.
func NewDinoTimeout(user *model.User, timeout time.Duration) *DinoTimeout {
	return &DinoTimeout{
		user:    user,
		timeout: timeout,
	}
}

// SetQueue sets the waiting queue.
func (t *DinoTimeout) SetQueue(queue QueueInterface) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.queue = queue
}

// Start begins the timeout countdown.
func (t *DinoTimeout) Start() {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.running = true
	t.canceled = false
	t.timer = time.AfterFunc(t.timeout, t.handleTimeout)
}

// Cancel stops the timeout (called when user completes the game).
func (t *DinoTimeout) Cancel() {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.canceled = true
	t.running = false
	if t.timer != nil {
		t.timer.Stop()
	}
}

// IsRunning returns whether the timeout is currently running.
func (t *DinoTimeout) IsRunning() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.running
}

// handleTimeout is called when the timeout expires.
func (t *DinoTimeout) handleTimeout() {
	t.mu.Lock()
	if t.canceled {
		t.mu.Unlock()
		return
	}
	t.running = false
	user := t.user
	t.mu.Unlock()

	// Send failure message
	if user.Conn != nil {
		_ = user.Conn.WriteJSON(map[string]interface{}{
			"type":           "failure",
			"message":        "タイムアウト！待機列の最後尾からやり直しです。",
			"redirect_delay": float64(3),
		})
	}

	// Reset user state
	user.ResetToWaiting()

	// Close WebSocket connection - user needs to reconnect fresh
	// Don't add to queue here - the user will be added when they reconnect via WebSocket
	if user.Conn != nil {
		conn := user.Conn // Capture for goroutine
		user.Conn = nil   // Clear the connection reference
		go func() {
			conn.Close()
		}()
	}
}
