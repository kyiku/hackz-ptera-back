// Package token provides token management for user registration.
package token

import (
	"sync"
	"time"

	"github.com/google/uuid"
	"hackz-ptera/back/internal/model"
)

// TokenExpiry is the duration after which a register token expires.
const TokenExpiry = 10 * time.Minute

// QueueInterface defines the interface for the waiting queue.
type QueueInterface interface {
	Add(userID string, conn *interface{})
}

// GenerateRegisterToken generates a new register token for the user.
// Sets the token and expiration time on the user.
func GenerateRegisterToken(user *model.User) string {
	token := uuid.New().String()
	user.RegisterToken = token
	user.RegisterTokenExp = time.Now().Add(TokenExpiry)
	return token
}

// ValidateRegisterToken validates the register token.
// Returns (valid, errorCode).
func ValidateRegisterToken(user *model.User, sessionID, token string) (bool, string) {
	// Check session ID
	if user.SessionID != sessionID {
		return false, "INVALID_SESSION"
	}

	// Check token
	if token == "" || user.RegisterToken != token {
		return false, "INVALID_TOKEN"
	}

	// Check expiration
	if IsTokenExpired(user) {
		return false, "TOKEN_EXPIRED"
	}

	return true, ""
}

// IsTokenExpired checks if the register token has expired.
func IsTokenExpired(user *model.User) bool {
	if user.RegisterTokenExp.IsZero() {
		return true
	}
	return time.Now().After(user.RegisterTokenExp) || time.Now().Equal(user.RegisterTokenExp)
}

// TokenMonitor monitors register tokens for expiration.
type TokenMonitor struct {
	mu            sync.Mutex
	checkInterval time.Duration
	queue         WaitingQueueInterface
	watchers      map[string]chan struct{} // userID -> stop channel
}

// WaitingQueueInterface defines the queue interface for token monitor.
type WaitingQueueInterface interface {
	Add(userID string, conn model.WebSocketConn)
}

// NewTokenMonitor creates a new token monitor.
func NewTokenMonitor(checkInterval time.Duration) *TokenMonitor {
	return &TokenMonitor{
		checkInterval: checkInterval,
		watchers:      make(map[string]chan struct{}),
	}
}

// SetQueue sets the waiting queue for the monitor.
func (m *TokenMonitor) SetQueue(queue WaitingQueueInterface) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.queue = queue
}

// Watch starts monitoring a user's token for expiration.
func (m *TokenMonitor) Watch(user *model.User) {
	m.mu.Lock()
	stopCh := make(chan struct{})
	m.watchers[user.ID] = stopCh
	m.mu.Unlock()

	go m.watchUser(user, stopCh)
}

// watchUser monitors a user's token in the background.
func (m *TokenMonitor) watchUser(user *model.User, stopCh chan struct{}) {
	ticker := time.NewTicker(m.checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-stopCh:
			return
		case <-ticker.C:
			if IsTokenExpired(user) {
				m.handleExpiration(user)
				return
			}
		}
	}
}

// handleExpiration handles token expiration for a user.
func (m *TokenMonitor) handleExpiration(user *model.User) {
	// Send notification via WebSocket
	if user.Conn != nil {
		_ = user.Conn.WriteJSON(map[string]interface{}{
			"code":    "TOKEN_EXPIRED",
			"message": "登録トークンの有効期限が切れました",
		})
		_ = user.Conn.Close()
	}

	// Reset user to waiting status
	user.ResetToWaiting()

	// Add back to queue
	m.mu.Lock()
	queue := m.queue
	m.mu.Unlock()

	if queue != nil {
		queue.Add(user.ID, user.Conn)
	}
}

// Unwatch stops monitoring a user's token.
func (m *TokenMonitor) Unwatch(user *model.User) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if stopCh, ok := m.watchers[user.ID]; ok {
		close(stopCh)
		delete(m.watchers, user.ID)
	}
}

// Stop stops all monitoring.
func (m *TokenMonitor) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, stopCh := range m.watchers {
		close(stopCh)
	}
	m.watchers = make(map[string]chan struct{})
}
