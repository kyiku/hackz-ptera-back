// Package handler provides HTTP handlers for the API.
package handler

import (
	"bytes"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"math"
	"math/rand"
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"hackz-ptera/back/internal/model"
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
		tolerance:     10, // default tolerance
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

// Generate creates a new CAPTCHA image.
func (h *CaptchaHandler) Generate(c echo.Context) error {
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
	if user.Status != "stage2_captcha" {
		return c.JSON(http.StatusForbidden, map[string]interface{}{
			"error":   true,
			"message": "CAPTCHAステージではありません",
		})
	}

	// Generate CAPTCHA image
	imgURL, targetX, targetY, err := h.generateCaptchaImage()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error":   true,
			"message": "CAPTCHA生成に失敗しました",
		})
	}

	// Save target position
	user.CaptchaTargetX = targetX
	user.CaptchaTargetY = targetY

	return c.JSON(http.StatusOK, map[string]interface{}{
		"error":     false,
		"image_url": imgURL,
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

	// Parse request
	var req VerifyRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error":   true,
			"message": "リクエストの解析に失敗しました",
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
	newImgURL, targetX, targetY, err := h.generateCaptchaImage()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error":   true,
			"message": "CAPTCHA再生成に失敗しました",
		})
	}

	user.CaptchaTargetX = targetX
	user.CaptchaTargetY = targetY
	remaining := model.MaxCaptchaAttempts - user.CaptchaAttempts

	return c.JSON(http.StatusOK, map[string]interface{}{
		"error":              true,
		"message":            "不正解です。もう一度試してください",
		"attempts_remaining": remaining,
		"new_image_url":      newImgURL,
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

// generateCaptchaImage creates a CAPTCHA image with hidden character.
func (h *CaptchaHandler) generateCaptchaImage() (string, int, int, error) {
	// Get background image
	bgKeys, err := h.s3Client.ListObjects("backgrounds/")
	if err != nil || len(bgKeys) == 0 {
		return "", 0, 0, fmt.Errorf("failed to list backgrounds: %w", err)
	}

	bgData, err := h.s3Client.GetObject(bgKeys[rand.Intn(len(bgKeys))])
	if err != nil {
		return "", 0, 0, fmt.Errorf("failed to get background: %w", err)
	}

	bgImg, _, err := image.Decode(bytes.NewReader(bgData))
	if err != nil {
		return "", 0, 0, fmt.Errorf("failed to decode background: %w", err)
	}

	// Get character image
	charData, err := h.s3Client.GetObject("character/char.png")
	if err != nil {
		return "", 0, 0, fmt.Errorf("failed to get character: %w", err)
	}

	charImg, _, err := image.Decode(bytes.NewReader(charData))
	if err != nil {
		return "", 0, 0, fmt.Errorf("failed to decode character: %w", err)
	}

	// Calculate random position
	bgBounds := bgImg.Bounds()
	charBounds := charImg.Bounds()

	maxX := bgBounds.Dx() - charBounds.Dx()
	maxY := bgBounds.Dy() - charBounds.Dy()

	if maxX <= 0 {
		maxX = 1
	}
	if maxY <= 0 {
		maxY = 1
	}

	targetX := rand.Intn(maxX)
	targetY := rand.Intn(maxY)

	// Compose image
	result := image.NewRGBA(bgBounds)
	draw.Draw(result, bgBounds, bgImg, bgBounds.Min, draw.Src)

	destRect := image.Rect(targetX, targetY, targetX+charBounds.Dx(), targetY+charBounds.Dy())
	draw.Draw(result, destRect, charImg, charBounds.Min, draw.Over)

	// Encode to PNG
	var buf bytes.Buffer
	if err := png.Encode(&buf, result); err != nil {
		return "", 0, 0, fmt.Errorf("failed to encode image: %w", err)
	}

	// Upload to S3
	filename := uuid.New().String() + ".png"
	key := "captcha/" + filename

	if err := h.s3Client.PutObject(key, buf.Bytes()); err != nil {
		return "", 0, 0, fmt.Errorf("failed to upload image: %w", err)
	}

	url := fmt.Sprintf("%s/%s", h.cloudfrontURL, key)
	return url, targetX, targetY, nil
}
