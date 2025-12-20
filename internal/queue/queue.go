// Package queue provides waiting queue management.
package queue

import (
	"sync"

	"github.com/kyiku/hackz-ptera-back/internal/model"
)

// QueueUser represents a user in the waiting queue.
type QueueUser struct {
	ID   string
	Conn model.WebSocketConn // WebSocket connection
}

// WaitingQueue manages users waiting in line.
type WaitingQueue struct {
	users []*QueueUser
	mu    sync.RWMutex
}

// NewWaitingQueue creates a new empty waiting queue.
func NewWaitingQueue() *WaitingQueue {
	return &WaitingQueue{
		users: make([]*QueueUser, 0),
	}
}

// Add adds a user to the end of the queue by userID and connection.
func (q *WaitingQueue) Add(userID string, conn model.WebSocketConn) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.users = append(q.users, &QueueUser{ID: userID, Conn: conn})
}

// AddUser adds a QueueUser to the end of the queue.
func (q *WaitingQueue) AddUser(user *QueueUser) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.users = append(q.users, user)
}

// Remove removes a user from the queue by ID.
func (q *WaitingQueue) Remove(userID string) {
	q.mu.Lock()
	defer q.mu.Unlock()

	for i, u := range q.users {
		if u.ID == userID {
			q.users = append(q.users[:i], q.users[i+1:]...)
			return
		}
	}
}

// GetPosition returns the position of a user in the queue (1-indexed).
// Returns 0 and false if the user is not found.
func (q *WaitingQueue) GetPosition(userID string) (int, bool) {
	q.mu.RLock()
	defer q.mu.RUnlock()

	for i, u := range q.users {
		if u.ID == userID {
			return i + 1, true
		}
	}
	return 0, false
}

// Len returns the number of users in the queue.
func (q *WaitingQueue) Len() int {
	q.mu.RLock()
	defer q.mu.RUnlock()
	return len(q.users)
}

// PopFront removes and returns the first user in the queue.
// Returns nil if the queue is empty.
func (q *WaitingQueue) PopFront() *QueueUser {
	q.mu.Lock()
	defer q.mu.Unlock()

	if len(q.users) == 0 {
		return nil
	}

	user := q.users[0]
	q.users = q.users[1:]
	return user
}

// BroadcastPositions sends position updates to all users in the queue.
func (q *WaitingQueue) BroadcastPositions() {
	q.mu.RLock()
	defer q.mu.RUnlock()

	total := len(q.users)
	for i, user := range q.users {
		if user.Conn != nil {
			_ = user.Conn.WriteJSON(map[string]interface{}{
				"type":     "queueUpdate",
				"position": i + 1,
				"total":    total,
			})
		}
	}
}
