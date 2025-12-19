// Package handler provides HTTP handlers for the API.
package handler

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"hackz-ptera/back/internal/ai"
)

// BedrockClientInterface defines the interface for Bedrock operations.
type BedrockClientInterface interface {
	InvokeModel(modelID string, prompt string) (string, error)
}

// PasswordHandler handles password analysis requests.
type PasswordHandler struct {
	store           SessionStoreInterface
	bedrockClient   *ai.BedrockClient
	fallbackEnabled bool
}

// NewPasswordHandler creates a new PasswordHandler.
func NewPasswordHandler(store SessionStoreInterface, bedrockClient BedrockClientInterface) *PasswordHandler {
	client := ai.NewBedrockClient(bedrockClient, "ap-northeast-1")
	return &PasswordHandler{
		store:         store,
		bedrockClient: client,
	}
}

// EnableFallback enables or disables fallback mode.
func (h *PasswordHandler) EnableFallback(enabled bool) {
	h.bedrockClient.EnableFallback(enabled)
}

// PasswordAnalyzeRequest represents the password analysis request.
type PasswordAnalyzeRequest struct {
	Password string `json:"password"`
}

// Analyze analyzes a password using AI.
func (h *PasswordHandler) Analyze(c echo.Context) error {
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
	var req PasswordAnalyzeRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error":   true,
			"message": "リクエストの解析に失敗しました",
		})
	}

	// Validate password
	if req.Password == "" {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error":   true,
			"message": "パスワードが空です",
		})
	}

	// Analyze password using Bedrock
	analysis, err := h.bedrockClient.AnalyzePassword(req.Password)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error":   true,
			"message": "パスワード分析に失敗しました",
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"error":    false,
		"analysis": analysis,
	})
}
