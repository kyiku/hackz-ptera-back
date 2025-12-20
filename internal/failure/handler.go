// Package failure provides failure handling for the application.
package failure

import (
	"github.com/kyiku/hackz-ptera-back/internal/model"
)

// QueueInterface defines the interface for the waiting queue.
type QueueInterface interface {
	Add(userID string, conn model.WebSocketConn)
}

// FailureHandler handles user failures and resets their state.
type FailureHandler struct {
	queue QueueInterface
}

// NewFailureHandler creates a new FailureHandler.
func NewFailureHandler(queue QueueInterface) *FailureHandler {
	return &FailureHandler{
		queue: queue,
	}
}

// HandleFailure processes a user failure, sends notification, closes connection,
// and adds the user back to the waiting queue.
func (h *FailureHandler) HandleFailure(user *model.User, message string) error {
	// Send failure message via WebSocket
	if user.Conn != nil {
		_ = user.Conn.WriteJSON(map[string]interface{}{
			"type":           "failure",
			"message":        message,
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

	return nil
}

// HandleCaptchaFailure handles CAPTCHA verification failure.
func (h *FailureHandler) HandleCaptchaFailure(user *model.User) error {
	return h.HandleFailure(user, "3回失敗しました。待機列の最後尾からやり直しです。")
}

// HandleDinoFailure handles Dino Run game failure.
func (h *FailureHandler) HandleDinoFailure(user *model.User) error {
	return h.HandleFailure(user, "ゲームオーバー。待機列の最後尾からやり直しです。")
}

// HandleOTPFailure handles OTP verification failure.
func (h *FailureHandler) HandleOTPFailure(user *model.User) error {
	return h.HandleFailure(user, "魚の名前を3回間違えました。")
}

// HandleTimeoutFailure handles timeout failures.
func (h *FailureHandler) HandleTimeoutFailure(user *model.User, message string) error {
	return h.HandleFailure(user, message)
}
