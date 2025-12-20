// Package handler provides HTTP handlers for the API.
package handler

import (
	"log"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/kyiku/hackz-ptera-back/internal/model"
)

// QueueInterfaceForDino is the queue interface for DinoHandler
type QueueInterfaceForDino interface {
	Remove(userID string)
	BroadcastPositions()
}

// DinoHandler handles Dino Run game related requests.
type DinoHandler struct {
	store SessionStoreInterface
	queue QueueInterfaceForDino
}

// NewDinoHandler creates a new DinoHandler.
func NewDinoHandler(store SessionStoreInterface) *DinoHandler {
	return &DinoHandler{
		store: store,
	}
}

// SetQueue sets the waiting queue.
func (h *DinoHandler) SetQueue(queue QueueInterfaceForDino) {
	h.queue = queue
}

// Start handles the game start request.
// This promotes the user from waiting to stage1_dino status.
func (h *DinoHandler) Start(c echo.Context) error {
	log.Println("[DinoHandler.Start] Request received")

	// Get session
	cookie, err := c.Cookie("session_id")
	if err != nil || cookie == nil {
		log.Printf("[DinoHandler.Start] No session cookie found: %v", err)
		return c.JSON(http.StatusOK, map[string]interface{}{
			"error":   true,
			"message": "セッションが見つかりません",
			"code":    "SESSION_NOT_FOUND",
		})
	}

	log.Printf("[DinoHandler.Start] Session cookie: %s", cookie.Value[:8]+"...")

	user, ok := h.store.Get(cookie.Value)
	if !ok {
		log.Printf("[DinoHandler.Start] Session not found in store: %s", cookie.Value[:8]+"...")
		return c.JSON(http.StatusOK, map[string]interface{}{
			"error":   true,
			"message": "無効なセッション",
			"code":    "INVALID_SESSION",
		})
	}

	log.Printf("[DinoHandler.Start] User found: %s, Status: %s", user.ID, user.Status)

	// Check if user is in waiting status (can be promoted)
	if user.Status != model.StatusWaiting {
		// Already promoted or in another stage - that's fine, just return success
		if user.Status == model.StatusStage1Dino {
			log.Printf("[DinoHandler.Start] User already in stage1_dino: %s", user.ID)
			return c.JSON(http.StatusOK, map[string]interface{}{
				"error":   false,
				"message": "ゲーム開始準備完了",
				"status":  user.Status,
			})
		}
		log.Printf("[DinoHandler.Start] User not in waiting status: %s (status=%s)", user.ID, user.Status)
		return c.JSON(http.StatusOK, map[string]interface{}{
			"error":   true,
			"message": "待機中ではありません",
			"code":    "NOT_WAITING",
		})
	}

	// Promote user to stage1_dino
	user.Status = model.StatusStage1Dino
	log.Printf("[DinoHandler.Start] User promoted to stage1_dino: %s", user.ID)

	// Remove from queue and broadcast to other users
	if h.queue != nil {
		h.queue.Remove(cookie.Value)
		h.queue.BroadcastPositions()
		log.Printf("[DinoHandler.Start] User removed from queue and positions broadcasted: %s", user.ID)
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"error":   false,
		"message": "ゲーム開始準備完了",
		"status":  user.Status,
	})
}

// DinoResultRequest represents the game result request.
type DinoResultRequest struct {
	Result string `json:"result"`
	Score  int    `json:"score"`
}

// Result handles the Dino Run game result.
func (h *DinoHandler) Result(c echo.Context) error {
	log.Println("[DinoHandler.Result] Request received")

	// Get session
	cookie, err := c.Cookie("session_id")
	if err != nil || cookie == nil {
		log.Printf("[DinoHandler.Result] No session cookie found: %v", err)
		return c.JSON(http.StatusOK, map[string]interface{}{
			"error":   true,
			"message": "セッションが見つかりません",
			"code":    "SESSION_NOT_FOUND",
		})
	}

	log.Printf("[DinoHandler.Result] Session cookie: %s", cookie.Value[:8]+"...")

	user, ok := h.store.Get(cookie.Value)
	if !ok {
		log.Printf("[DinoHandler.Result] Session not found in store: %s", cookie.Value[:8]+"...")
		return c.JSON(http.StatusOK, map[string]interface{}{
			"error":   true,
			"message": "無効なセッション",
			"code":    "INVALID_SESSION",
		})
	}

	log.Printf("[DinoHandler.Result] User found: %s, Status: %s", user.ID, user.Status)

	// Check user status
	if user.Status != "stage1_dino" {
		log.Printf("[DinoHandler.Result] WRONG_STAGE: User %s has status %s (expected stage1_dino)", user.ID, user.Status)
		return c.JSON(http.StatusOK, map[string]interface{}{
			"error":   true,
			"message": "Dino Runステージではありません",
			"code":    "WRONG_STAGE",
		})
	}

	// Parse request
	var req DinoResultRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"error":   true,
			"message": "リクエストの解析に失敗しました",
			"code":    "BAD_REQUEST",
		})
	}

	log.Printf("[DinoHandler.Result] Game result: %s, Score: %d", req.Result, req.Score)

	// Handle result
	if req.Result == "clear" {
		// Success - advance to registration dashboard (hub & spoke)
		user.Status = model.StatusRegistering
		log.Printf("[DinoHandler.Result] User %s cleared! Status changed to registering", user.ID)
		return c.JSON(http.StatusOK, map[string]interface{}{
			"error":      false,
			"next_stage": "register",
			"message":    "ゲームクリア！登録フォームに進みます",
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
