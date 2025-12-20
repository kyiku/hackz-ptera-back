// Package handler provides HTTP handlers for the API.
package handler

import (
	"fmt"
	"math/rand"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/kyiku/hackz-ptera-back/internal/model"
	"github.com/kyiku/hackz-ptera-back/internal/util"
)

// OTPHandler handles OTP-related requests.
type OTPHandler struct {
	store         SessionStoreInterface
	s3Client      S3ClientInterface
	queue         QueueInterfaceForCaptcha
	cloudfrontURL string
}

// NewOTPHandler creates a new OTPHandler.
func NewOTPHandler(store SessionStoreInterface, s3Client S3ClientInterface) *OTPHandler {
	return &OTPHandler{
		store:         store,
		s3Client:      s3Client,
		cloudfrontURL: "https://test.cloudfront.net",
	}
}

// SetQueue sets the waiting queue.
func (h *OTPHandler) SetQueue(queue QueueInterfaceForCaptcha) {
	h.queue = queue
}

// predefinedFish contains the list of fish for OTP.
var predefinedFish = []struct {
	Name     string
	Filename string
}{
	{Name: "オニカマス", Filename: "onikamasu"},
	{Name: "ホウボウ", Filename: "houhou"},
	{Name: "マツカサウオ", Filename: "matsukasauo"},
	{Name: "ハリセンボン", Filename: "harisenbon"},
	{Name: "カワハギ", Filename: "kawahagi"},
	{Name: "フグ", Filename: "fugu"},
	{Name: "タツノオトシゴ", Filename: "tatsunootoshigo"},
	{Name: "オコゼ", Filename: "okoze"},
	{Name: "アンコウ", Filename: "ankou"},
	{Name: "ウツボ", Filename: "utsubo"},
}

// Send generates and sends an OTP fish image.
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

	// Select random fish
	fish := predefinedFish[rand.Intn(len(predefinedFish))]

	// Save fish name for verification
	user.OTPFishName = fish.Name
	user.OTPAttempts = 0

	// Generate image URL
	imageURL := fmt.Sprintf("%s/fish/%s.jpg", h.cloudfrontURL, fish.Filename)

	return c.JSON(http.StatusOK, map[string]interface{}{
		"error":     false,
		"image_url": imageURL,
		"message":   "この魚の名前を入力してください",
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

	// Check answer using kana-insensitive matching
	if util.KanaMatch(req.Answer, user.OTPFishName) {
		// Success - registration complete
		return c.JSON(http.StatusOK, map[string]interface{}{
			"error":   false,
			"message": "正解です！登録が完了しました",
		})
	}

	// Failed attempt
	exceeded := user.IncrementOTPAttempts()

	if exceeded {
		// 3 failures - reset to waiting
		return h.handleMaxAttempts(c, user)
	}

	// Generate new fish for retry
	fish := h.getRandomFishExcluding(user.OTPFishName)
	user.OTPFishName = fish.Name

	remaining := model.MaxOTPAttempts - user.OTPAttempts
	imageURL := fmt.Sprintf("%s/fish/%s.jpg", h.cloudfrontURL, fish.Filename)

	return c.JSON(http.StatusOK, map[string]interface{}{
		"error":              true,
		"message":            "不正解です。もう一度試してください",
		"attempts_remaining": remaining,
		"new_image_url":      imageURL,
	})
}

// handleMaxAttempts handles the case when max OTP attempts are exceeded.
func (h *OTPHandler) handleMaxAttempts(c echo.Context, user *model.User) error {
	// Send failure notification via WebSocket
	if user.Conn != nil {
		_ = user.Conn.WriteJSON(map[string]interface{}{
			"type":           "failure",
			"message":        "魚の名前を3回間違えました。",
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
		"message":        "魚の名前を3回間違えました。",
		"redirect_delay": float64(3),
	})
}

// getRandomFishExcluding returns a random fish excluding the specified name.
func (h *OTPHandler) getRandomFishExcluding(excludeName string) struct {
	Name     string
	Filename string
} {
	for {
		fish := predefinedFish[rand.Intn(len(predefinedFish))]
		if fish.Name != excludeName {
			return fish
		}
	}
}
