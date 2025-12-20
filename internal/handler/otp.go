// Package handler provides HTTP handlers for the API.
package handler

import (
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"
	"github.com/kyiku/hackz-ptera-back/internal/calculus"
	"github.com/kyiku/hackz-ptera-back/internal/model"
	"github.com/kyiku/hackz-ptera-back/internal/token"
)

// OTPHandler handles OTP-related requests.
type OTPHandler struct {
	store         SessionStoreInterface
	queue         QueueInterfaceForCaptcha
	calcGenerator *calculus.Generator
}

// NewOTPHandler creates a new OTPHandler.
func NewOTPHandler(store SessionStoreInterface, s3Client S3ClientInterface) *OTPHandler {
	return &OTPHandler{
		store:         store,
		calcGenerator: calculus.NewGenerator(),
	}
}

// SetQueue sets the waiting queue.
func (h *OTPHandler) SetQueue(queue QueueInterfaceForCaptcha) {
	h.queue = queue
}

// Send generates and returns a calculus problem.
func (h *OTPHandler) Send(c echo.Context) error {
	// Get session
	cookie, err := c.Cookie("session_id")
	if err != nil || cookie == nil {
		// CloudFrontのcustom_error_responseがHTMLを返すのを防ぐため、常に200を返す
		return c.JSON(http.StatusOK, map[string]interface{}{
			"error":   true,
			"message": "セッションが見つかりません",
			"code":    "SESSION_NOT_FOUND",
		})
	}

	user, ok := h.store.Get(cookie.Value)
	if !ok {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"error":   true,
			"message": "無効なセッション",
			"code":    "INVALID_SESSION",
		})
	}

	// Check user status
	if user.Status != "registering" {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"error":   true,
			"message": "登録ステージではありません",
			"code":    "WRONG_STAGE",
		})
	}

	// Generate calculus problem
	problem, err := h.calcGenerator.Generate()
	if err != nil {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"error":   true,
			"message": "問題生成に失敗しました",
			"code":    "GENERATION_FAILED",
		})
	}

	// Save OTP for verification
	user.OTPCode = problem.OTP
	user.OTPAttempts = 0

	return c.JSON(http.StatusOK, map[string]interface{}{
		"error":         false,
		"problem_latex": problem.ProblemLatex,
		"message":       "微分問題を解いて6桁の答えを入力してください",
	})
}

// OTPVerifyRequest represents the OTP verification request.
type OTPVerifyRequest struct {
	Answer string `json:"answer"`
}

// Verify checks the OTP answer.
func (h *OTPHandler) Verify(c echo.Context) error {
	// Get session
	cookie, err := c.Cookie("session_id")
	if err != nil || cookie == nil {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"error":   true,
			"message": "セッションが見つかりません",
			"code":    "SESSION_NOT_FOUND",
		})
	}

	user, ok := h.store.Get(cookie.Value)
	if !ok {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"error":   true,
			"message": "無効なセッション",
			"code":    "INVALID_SESSION",
		})
	}

	// Parse request
	var req OTPVerifyRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"error":   true,
			"message": "リクエストの解析に失敗しました",
			"code":    "BAD_REQUEST",
		})
	}

	// Parse answer as integer
	answer, err := strconv.Atoi(req.Answer)
	if err != nil {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"error":   true,
			"message": "6桁の数字を入力してください",
			"code":    "INVALID_ANSWER",
		})
	}

	// Check answer
	if answer == user.OTPCode {
		// Success - generate registration token
		registerToken := token.GenerateRegisterToken(user)

		return c.JSON(http.StatusOK, map[string]interface{}{
			"error":          false,
			"message":        "正解です！登録が完了しました",
			"register_token": registerToken,
		})
	}

	// Failed attempt
	exceeded := user.IncrementOTPAttempts()

	if exceeded {
		// 3 failures - reset to waiting
		return h.handleMaxAttempts(c, user)
	}

	// Generate new problem for retry
	newProblem, _ := h.calcGenerator.Generate()
	user.OTPCode = newProblem.OTP

	remaining := model.MaxOTPAttempts - user.OTPAttempts

	return c.JSON(http.StatusOK, map[string]interface{}{
		"error":              true,
		"message":            "不正解です。もう一度試してください",
		"attempts_remaining": remaining,
		"new_problem_latex":  newProblem.ProblemLatex,
	})
}

// handleMaxAttempts handles the case when max OTP attempts are exceeded.
func (h *OTPHandler) handleMaxAttempts(c echo.Context, user *model.User) error {
	// Send failure notification via WebSocket
	if user.Conn != nil {
		_ = user.Conn.WriteJSON(map[string]interface{}{
			"type":           "failure",
			"message":        "3回失敗しました。待機列の最後尾からやり直しです。",
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

	return c.JSON(http.StatusOK, map[string]interface{}{
		"error":          true,
		"message":        "3回失敗しました。待機列の最後尾からやり直しです。",
		"redirect_delay": float64(3),
	})
}
