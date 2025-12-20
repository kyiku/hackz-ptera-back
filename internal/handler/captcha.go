// Package handler provides HTTP handlers for the API.
package handler

import (
	"fmt"
	"math"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/kyiku/hackz-ptera-back/internal/captcha"
	"github.com/kyiku/hackz-ptera-back/internal/model"
)

// SessionStoreInterface defines the interface for session storage.
type SessionStoreInterface interface {
	Get(sessionID string) (*model.User, bool)
}

// S3ClientInterface defines the interface for S3 operations.
type S3ClientInterface interface {
	GetObject(key string) ([]byte, error)
	PutObject(key string, data []byte) error
	ListObjects(prefix string) ([]string, error)
}

// QueueInterfaceForCaptcha defines the queue interface for CAPTCHA handler.
type QueueInterfaceForCaptcha interface {
	Add(userID string, conn model.WebSocketConn)
}

// CaptchaHandler handles CAPTCHA-related requests.
type CaptchaHandler struct {
	store         SessionStoreInterface
	s3Client      S3ClientInterface
	queue         QueueInterfaceForCaptcha
	tolerance     int
	cloudfrontURL string
}

// NewCaptchaHandler creates a new CaptchaHandler.
func NewCaptchaHandler(store SessionStoreInterface, s3Client S3ClientInterface) *CaptchaHandler {
	return &CaptchaHandler{
		store:         store,
		s3Client:      s3Client,
		tolerance:     25, // default tolerance (half of 50x50 character size)
		cloudfrontURL: "https://test.cloudfront.net",
	}
}

// SetQueue sets the waiting queue.
func (h *CaptchaHandler) SetQueue(queue QueueInterfaceForCaptcha) {
	h.queue = queue
}

// SetTolerance sets the click tolerance in pixels.
func (h *CaptchaHandler) SetTolerance(tolerance int) {
	h.tolerance = tolerance
}

// SetCloudfrontURL sets the CloudFront URL for image delivery.
func (h *CaptchaHandler) SetCloudfrontURL(url string) {
	h.cloudfrontURL = url
}

// Generate creates a new CAPTCHA image.
func (h *CaptchaHandler) Generate(c echo.Context) error {
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

	// Check user status - CAPTCHA is one of the 9 tasks in "registering" stage
	if user.Status != "registering" {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"error":   true,
			"message": "登録ステージではありません",
			"code":    "WRONG_STAGE",
		})
	}

	// Generate CAPTCHA image
	result, err := h.generateCaptchaImage()
	if err != nil {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"error":   true,
			"message": "CAPTCHA生成に失敗しました",
			"code":    "GENERATION_FAILED",
		})
	}

	// Save target position
	user.CaptchaTargetX = result.TargetX
	user.CaptchaTargetY = result.TargetY

	return c.JSON(http.StatusOK, map[string]interface{}{
		"error":            false,
		"image_url":        result.ImageURL,
		"target_image_url": result.TargetImageURL,
	})
}

// VerifyRequest represents the CAPTCHA verification request.
type VerifyRequest struct {
	X int `json:"x"`
	Y int `json:"y"`
}

// Verify checks the CAPTCHA answer.
func (h *CaptchaHandler) Verify(c echo.Context) error {
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
	var req VerifyRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"error":   true,
			"message": "リクエストの解析に失敗しました",
			"code":    "BAD_REQUEST",
		})
	}

	// Check if click is within tolerance
	dx := float64(req.X - user.CaptchaTargetX)
	dy := float64(req.Y - user.CaptchaTargetY)
	distance := math.Sqrt(dx*dx + dy*dy)

	if distance <= float64(h.tolerance) {
		// Success - advance to registering stage
		user.Status = "registering"
		return c.JSON(http.StatusOK, map[string]interface{}{
			"error":      false,
			"next_stage": "registering",
			"message":    "CAPTCHA成功！登録フォームに進みます",
		})
	}

	// Failed attempt
	exceeded := user.IncrementCaptchaAttempts()

	if exceeded {
		// 3 failures - reset to waiting
		return h.handleMaxAttempts(c, user)
	}

	// Generate new CAPTCHA for retry
	newResult, err := h.generateCaptchaImage()
	if err != nil {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"error":   true,
			"message": "CAPTCHA再生成に失敗しました",
			"code":    "REGENERATION_FAILED",
		})
	}

	user.CaptchaTargetX = newResult.TargetX
	user.CaptchaTargetY = newResult.TargetY
	remaining := model.MaxCaptchaAttempts - user.CaptchaAttempts

	return c.JSON(http.StatusOK, map[string]interface{}{
		"error":                  true,
		"message":                "不正解です。もう一度試してください",
		"attempts_remaining":     remaining,
		"new_image_url":          newResult.ImageURL,
		"new_target_image_url":   newResult.TargetImageURL,
	})
}

// handleMaxAttempts handles the case when max attempts are exceeded.
func (h *CaptchaHandler) handleMaxAttempts(c echo.Context, user *model.User) error {
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

	// Add back to queue
	if h.queue != nil {
		h.queue.Add(user.ID, user.Conn)
	}

	// Close connection after sending message
	if user.Conn != nil {
		go user.Conn.Close()
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"error":          true,
		"message":        "3回失敗しました。待機列の最後尾からやり直しです。",
		"redirect_delay": float64(3),
	})
}

// CaptchaImageResult holds the result of CAPTCHA image generation.
type CaptchaImageResult struct {
	ImageURL       string
	TargetImageURL string
	TargetX        int
	TargetY        int
}

// generateCaptchaImage creates a CAPTCHA image with multiple characters.
// Returns the image URL, target image URL, and target center coordinates.
func (h *CaptchaHandler) generateCaptchaImage() (*CaptchaImageResult, error) {
	gen := captcha.NewGenerator(h.s3Client, h.cloudfrontURL)

	result, err := gen.GenerateMultiCharacter()
	if err != nil {
		return nil, fmt.Errorf("failed to generate captcha: %w", err)
	}

	url, err := gen.Upload(result.Image)
	if err != nil {
		return nil, fmt.Errorf("failed to upload captcha: %w", err)
	}

	return &CaptchaImageResult{
		ImageURL:       url,
		TargetImageURL: result.TargetImageURL,
		TargetX:        result.TargetX,
		TargetY:        result.TargetY,
	}, nil
}
