// Package stage provides stage transition management.
package stage

import (
	"errors"

	"hackz-ptera/back/internal/model"
)

// stageMessages contains the WebSocket messages for each stage.
var stageMessages = map[string]string{
	"stage1_dino":    "Dino Run ゲームを開始してください",
	"stage2_captcha": "CAPTCHAを解いてください",
	"registering":    "登録フォームに入力してください",
}

// TransitionManager manages stage transitions.
type TransitionManager struct{}

// NewTransitionManager creates a new TransitionManager.
func NewTransitionManager() *TransitionManager {
	return &TransitionManager{}
}

// CanTransition checks if the user can transition to the target status.
// Returns (valid, errorCode).
func (m *TransitionManager) CanTransition(user *model.User, toStatus string) (bool, string) {
	if user.CanTransitionTo(toStatus) {
		return true, ""
	}
	return false, "INVALID_TRANSITION"
}

// Execute performs the stage transition and notifies the user.
func (m *TransitionManager) Execute(user *model.User, toStatus string) error {
	valid, errCode := m.CanTransition(user, toStatus)
	if !valid {
		return errors.New(errCode)
	}

	// Update user status
	user.Status = toStatus

	// Send WebSocket notification
	if user.Conn != nil {
		message, ok := stageMessages[toStatus]
		if !ok {
			message = "ステージが変更されました"
		}

		user.Conn.WriteJSON(map[string]interface{}{
			"type":    "stage_change",
			"stage":   toStatus,
			"message": message,
		})
	}

	return nil
}
