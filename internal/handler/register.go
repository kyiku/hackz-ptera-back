// Package handler provides HTTP handlers for the API.
package handler

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/kyiku/hackz-ptera-back/internal/model"
)

// RegisterHandler handles registration requests.
type RegisterHandler struct {
	store SessionStoreInterface
	queue QueueInterfaceForCaptcha
}

// NewRegisterHandler creates a new RegisterHandler.
func NewRegisterHandler(store SessionStoreInterface) *RegisterHandler {
	return &RegisterHandler{
		store: store,
	}
}

// SetQueue sets the waiting queue.
func (h *RegisterHandler) SetQueue(queue QueueInterfaceForCaptcha) {
	h.queue = queue
}

// RegisterRequest represents the registration request.
type RegisterRequest struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
	Token    string `json:"token"`
}

// Submit handles the registration form submission.
// This is the "evil" handler - it ALWAYS fails with a server error.
func (h *RegisterHandler) Submit(c echo.Context) error {
	// Get session
	cookie, err := c.Cookie("session_id")
	if err != nil || cookie == nil {
		return c.JSON(http.StatusUnauthorized, map[string]interface{}{
			"error":   true,
			"message": "セッションが見つかりません",
		})
	}

	user, ok := h.store.Get(cookie.Value)
	if !ok {
		return c.JSON(http.StatusUnauthorized, map[string]interface{}{
			"error":   true,
			"message": "無効なセッション",
		})
	}

	// Check user status
	if user.Status != "registering" {
		return c.JSON(http.StatusForbidden, map[string]interface{}{
			"error":   true,
			"message": "登録ステージではありません",
		})
	}

	// Parse request
	var req RegisterRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error":   true,
			"message": "リクエストの解析に失敗しました",
		})
	}

	// EVIL: Always fail with server error
	// This is the joke - the registration never succeeds
	return h.handleFakeServerError(c, user)
}

// handleFakeServerError simulates a server error and resets the user.
func (h *RegisterHandler) handleFakeServerError(c echo.Context, user *model.User) error {
	// Send failure notification via WebSocket
	if user.Conn != nil {
		_ = user.Conn.WriteJSON(map[string]interface{}{
			"type":           "failure",
			"message":        "サーバーエラーが発生しました。待機列の最後尾からやり直しです。",
			"redirect_delay": float64(3),
		})
	}

	// Reset user state
	user.ResetToWaiting()

	// Add back to queue
	if h.queue != nil {
		h.queue.Add(user.ID, user.Conn)
	}

	// Close connection after sending message
	if user.Conn != nil {
		go user.Conn.Close()
	}

	return c.JSON(http.StatusInternalServerError, map[string]interface{}{
		"error":          true,
		"message":        "サーバーエラーが発生しました。待機列の最後尾からやり直しです。",
		"redirect_delay": float64(3),
	})
}
