// Package game provides game-related functionality like timeouts.
package game

import (
	"sync"
	"time"

	"hackz-ptera/back/internal/model"
)

// CaptchaTimeout manages the CAPTCHA challenge timeout.
type CaptchaTimeout struct {
	mu       sync.Mutex
	user     *model.User
	timeout  time.Duration
	timer    *time.Timer
	running  bool
	canceled bool
	queue    QueueInterface
}

// NewCaptchaTimeout creates a new CaptchaTimeout for a user.
func NewCaptchaTimeout(user *model.User, timeout time.Duration) *CaptchaTimeout {
	return &CaptchaTimeout{
		user:    user,
		timeout: timeout,
	}
}

// SetQueue sets the waiting queue.
func (t *CaptchaTimeout) SetQueue(queue QueueInterface) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.queue = queue
}

// Start begins the timeout countdown.
func (t *CaptchaTimeout) Start() {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.running = true
	t.canceled = false
	t.timer = time.AfterFunc(t.timeout, t.handleTimeout)
}

// Cancel stops the timeout (called when user completes the CAPTCHA).
func (t *CaptchaTimeout) Cancel() {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.canceled = true
	t.running = false
	if t.timer != nil {
		t.timer.Stop()
	}
}

// IsRunning returns whether the timeout is currently running.
func (t *CaptchaTimeout) IsRunning() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.running
}

// handleTimeout is called when the timeout expires.
func (t *CaptchaTimeout) handleTimeout() {
	t.mu.Lock()
	if t.canceled {
		t.mu.Unlock()
		return
	}
	t.running = false
	user := t.user
	queue := t.queue
	t.mu.Unlock()

	// Send failure message
	if user.Conn != nil {
		user.Conn.WriteJSON(map[string]interface{}{
			"type":           "failure",
			"message":        "タイムアウト！待機列の最後尾からやり直しです。",
			"redirect_delay": float64(3),
		})
	}

	// Reset user state
	user.ResetToWaiting()

	// Add back to queue
	if queue != nil {
		queue.Add(user.ID, user.Conn)
	}

	// Close connection
	if user.Conn != nil {
		user.Conn.Close()
	}
}
