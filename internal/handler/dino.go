// Package handler provides HTTP handlers for the API.
package handler

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/kyiku/hackz-ptera-back/internal/model"
)

// DinoHandler handles Dino Run game related requests.
type DinoHandler struct {
	store SessionStoreInterface
}

// NewDinoHandler creates a new DinoHandler.
func NewDinoHandler(store SessionStoreInterface) *DinoHandler {
	return &DinoHandler{
		store: store,
	}
}

// DinoResultRequest represents the game result request.
type DinoResultRequest struct {
	Result string `json:"result"`
	Score  int    `json:"score"`
}

// Result handles the Dino Run game result.
func (h *DinoHandler) Result(c echo.Context) error {
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
	if user.Status != "stage1_dino" {
		return c.JSON(http.StatusForbidden, map[string]interface{}{
			"error":   true,
			"message": "Dino Runステージではありません",
		})
	}

	// Parse request
	var req DinoResultRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error":   true,
			"message": "リクエストの解析に失敗しました",
		})
	}

	// Handle result
	if req.Result == "clear" {
		// Success - advance to CAPTCHA stage
		user.Status = model.StatusStage2Captcha
		return c.JSON(http.StatusOK, map[string]interface{}{
			"error":      false,
			"next_stage": "stage2_captcha",
			"message":    "ゲームクリア！CAPTCHAステージに進みます",
			"score":      req.Score,
		})
	}

	// Game over - reset to waiting
	user.ResetToWaiting()

	// Send failure notification via WebSocket
	if user.Conn != nil {
		_ = user.Conn.WriteJSON(map[string]interface{}{
			"type":           "failure",
			"message":        "ゲームオーバー。待機列の最後尾からやり直しです。",
			"redirect_delay": float64(3),
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"error":          true,
		"message":        "ゲームオーバー。待機列の最後尾からやり直しです。",
		"redirect_delay": float64(3),
	})
}
