// Package model provides data models for the application.
package model

import (
	"time"

	"github.com/google/uuid"
)

// Status constants for user state
const (
	StatusWaiting       = "waiting"
	StatusStage1Dino    = "stage1_dino"
	StatusStage2Captcha = "stage2_captcha"
	StatusRegistering   = "registering"
)

// MaxCaptchaAttempts is the maximum number of CAPTCHA attempts allowed.
const MaxCaptchaAttempts = 3

// MaxOTPAttempts is the maximum number of OTP attempts allowed.
const MaxOTPAttempts = 3

// WebSocketConn defines the interface for WebSocket connections.
type WebSocketConn interface {
	WriteMessage(messageType int, data []byte) error
	WriteJSON(v interface{}) error
	Close() error
}

// User represents a user in the system.
type User struct {
	ID        string    // UUID
	SessionID string    // Session ID (Cookie)
	JoinedAt  time.Time // When the user joined the queue
	Status    string    // Current status

	// CAPTCHA fields
	CaptchaTargetX  int // Target X coordinate for CAPTCHA
	CaptchaTargetY  int // Target Y coordinate for CAPTCHA
	CaptchaAttempts int // Number of CAPTCHA attempts (max 3)

	// OTP fields
	OTPFishName string // Correct fish name for OTP
	OTPAttempts int    // Number of OTP attempts (max 3)

	// Registration fields
	RegisterToken    string    // Registration token (UUID)
	RegisterTokenExp time.Time // Token expiration time (10 minutes)

	// WebSocket connection
	Conn WebSocketConn // WebSocket connection for real-time communication
}

// NewUser creates a new User with default values.
func NewUser() *User {
	return &User{
		ID:       uuid.New().String(),
		Status:   StatusWaiting,
		JoinedAt: time.Now(),
	}
}

// validTransitions defines allowed status transitions.
var validTransitions = map[string][]string{
	StatusWaiting:       {StatusStage1Dino},
	StatusStage1Dino:    {StatusStage2Captcha, StatusWaiting},
	StatusStage2Captcha: {StatusRegistering, StatusWaiting},
	StatusRegistering:   {StatusWaiting},
}

// CanTransitionTo checks if the user can transition to the given status.
func (u *User) CanTransitionTo(status string) bool {
	allowedStatuses, ok := validTransitions[u.Status]
	if !ok {
		return false
	}

	for _, allowed := range allowedStatuses {
		if allowed == status {
			return true
		}
	}
	return false
}

// ResetToWaiting resets the user's state to waiting.
// This is called when the user fails at any stage.
func (u *User) ResetToWaiting() {
	u.Status = StatusWaiting

	// Reset CAPTCHA state
	u.CaptchaAttempts = 0
	u.CaptchaTargetX = 0
	u.CaptchaTargetY = 0

	// Reset OTP state
	u.OTPAttempts = 0
	u.OTPFishName = ""

	// Reset registration token
	u.RegisterToken = ""
	u.RegisterTokenExp = time.Time{}
}

// SetCaptchaTarget sets the CAPTCHA target coordinates.
func (u *User) SetCaptchaTarget(x, y int) {
	u.CaptchaTargetX = x
	u.CaptchaTargetY = y
}

// IncrementCaptchaAttempts increments the CAPTCHA attempt count.
// Returns true if the maximum attempts have been exceeded.
func (u *User) IncrementCaptchaAttempts() bool {
	u.CaptchaAttempts++
	return u.CaptchaAttempts >= MaxCaptchaAttempts
}

// IncrementOTPAttempts increments the OTP attempt count.
// Returns true if the maximum attempts have been exceeded.
func (u *User) IncrementOTPAttempts() bool {
	u.OTPAttempts++
	return u.OTPAttempts >= MaxOTPAttempts
}
