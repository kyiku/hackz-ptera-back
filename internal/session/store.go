// Package session provides session management functionality.
package session

import (
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/kyiku/hackz-ptera-back/internal/model"
)

// sessionEntry holds a user and its creation time for expiry checking.
type sessionEntry struct {
	User      *model.User
	CreatedAt time.Time
}

// SessionStore manages user sessions in memory.
type SessionStore struct {
	sessions map[string]*sessionEntry
	mu       sync.RWMutex
	expiry   time.Duration // 0 means no expiry
}

// NewSessionStore creates a new SessionStore with no expiry.
func NewSessionStore() *SessionStore {
	return &SessionStore{
		sessions: make(map[string]*sessionEntry),
		expiry:   0,
	}
}

// NewSessionStoreWithExpiry creates a new SessionStore with the specified expiry duration.
func NewSessionStoreWithExpiry(expiry time.Duration) *SessionStore {
	return &SessionStore{
		sessions: make(map[string]*sessionEntry),
		expiry:   expiry,
	}
}

// Create creates a new session and returns the user and session ID.
func (s *SessionStore) Create() (*model.User, string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	user := model.NewUser()
	sessionID := uuid.New().String()
	user.SessionID = sessionID

	s.sessions[sessionID] = &sessionEntry{
		User:      user,
		CreatedAt: time.Now(),
	}

	return user, sessionID
}

// Get retrieves a user by session ID.
// Returns nil and false if the session does not exist or has expired.
func (s *SessionStore) Get(sessionID string) (*model.User, bool) {
	s.mu.RLock()
	entry, exists := s.sessions[sessionID]
	s.mu.RUnlock()

	if !exists {
		return nil, false
	}

	// Check expiry if set
	if s.expiry > 0 && time.Since(entry.CreatedAt) > s.expiry {
		// Session has expired, delete it
		s.mu.Lock()
		delete(s.sessions, sessionID)
		s.mu.Unlock()
		return nil, false
	}

	return entry.User, true
}

// Delete removes a session by ID.
func (s *SessionStore) Delete(sessionID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, sessionID)
}

// Count returns the number of active sessions.
func (s *SessionStore) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.sessions)
}
