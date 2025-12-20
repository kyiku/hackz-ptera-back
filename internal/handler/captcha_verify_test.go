package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/kyiku/hackz-ptera-back/internal/model"
	"github.com/kyiku/hackz-ptera-back/internal/queue"
	"github.com/kyiku/hackz-ptera-back/internal/session"
	"github.com/kyiku/hackz-ptera-back/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCaptchaHandler_Verify(t *testing.T) {
	tests := []struct {
		name           string
		setupUser      func(*model.User)
		clickX         int
		clickY         int
		targetX        int
		targetY        int
		tolerance      int
		hasCookie      bool
		wantStatusCode int
		wantError      bool
		wantNextStage  string
	}{
		{
			name: "正常系: 正確なクリック",
			setupUser: func(u *model.User) {
				u.Status = "registering"
				u.CaptchaTargetX = 512
				u.CaptchaTargetY = 384
				u.CaptchaAttempts = 0
			},
			clickX:         512,
			clickY:         384,
			targetX:        512,
			targetY:        384,
			tolerance:      10,
			hasCookie:      true,
			wantStatusCode: http.StatusOK,
			wantError:      false,
			wantNextStage:  "",
		},
		{
			name: "正常系: 許容範囲内のクリック",
			setupUser: func(u *model.User) {
				u.Status = "registering"
				u.CaptchaTargetX = 512
				u.CaptchaTargetY = 384
				u.CaptchaAttempts = 0
			},
			clickX:         515,
			clickY:         380,
			targetX:        512,
			targetY:        384,
			tolerance:      10,
			hasCookie:      true,
			wantStatusCode: http.StatusOK,
			wantError:      false,
			wantNextStage:  "",
		},
		{
			name: "異常系: 許容範囲外のクリック（1回目）",
			setupUser: func(u *model.User) {
				u.Status = "registering"
				u.CaptchaTargetX = 512
				u.CaptchaTargetY = 384
				u.CaptchaAttempts = 0
			},
			clickX:         100,
			clickY:         100,
			targetX:        512,
			targetY:        384,
			tolerance:      10,
			hasCookie:      true,
			wantStatusCode: http.StatusOK,
			wantError:      true,
			wantNextStage:  "",
		},
		{
			name:           "異常系: セッションなし",
			setupUser:      nil,
			clickX:         512,
			clickY:         384,
			targetX:        0,
			targetY:        0,
			tolerance:      10,
			hasCookie:      false,
			wantStatusCode: http.StatusUnauthorized,
			wantError:      true,
			wantNextStage:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := session.NewSessionStore()
			mockS3 := testutil.NewMockS3Client()
			mockS3.Objects = map[string][]byte{
				"backgrounds/bg1.png":  testutil.CreateTestPNG(2816, 1536),
				"character/char1.png":  testutil.CreateTestPNG(100, 100),
				"character/char2.png":  testutil.CreateTestPNG(100, 100),
				"character/char3.png":  testutil.CreateTestPNG(100, 100),
				"character/char4.png":  testutil.CreateTestPNG(100, 100),
			}

			var sessionID string
			var user *model.User
			if tt.setupUser != nil {
				user, sessionID = store.Create()
				tt.setupUser(user)
			}

			h := NewCaptchaHandler(store, mockS3)
			h.SetTolerance(tt.tolerance)

			body := `{"x": ` + itoa(tt.clickX) + `, "y": ` + itoa(tt.clickY) + `}`
			tc := testutil.NewTestContext(http.MethodPost, "/api/captcha/verify", strings.NewReader(body))
			tc.Request.Header.Set("Content-Type", "application/json")
			if tt.hasCookie && sessionID != "" {
				tc.Request.AddCookie(&http.Cookie{Name: "session_id", Value: sessionID})
			}

			err := h.Verify(tc.Context)

			require.NoError(t, err)
			assert.Equal(t, tt.wantStatusCode, tc.Recorder.Code)

			var resp map[string]interface{}
			_ = json.Unmarshal(tc.Recorder.Body.Bytes(), &resp)

			assert.Equal(t, tt.wantError, resp["error"])

			if tt.wantNextStage != "" {
				assert.Equal(t, tt.wantNextStage, resp["nextStage"])
			}
		})
	}
}

func TestCaptchaHandler_Verify_ThreeFailures(t *testing.T) {
	store := session.NewSessionStore()
	q := queue.NewWaitingQueue()
	mockS3 := testutil.NewMockS3Client()

	mockConn := testutil.NewMockWebSocketConn()
	user, sessionID := store.Create()
	user.Status = "registering"
	user.CaptchaTargetX = 512
	user.CaptchaTargetY = 384
	user.CaptchaAttempts = 2 // 既に2回失敗
	user.Conn = mockConn

	h := NewCaptchaHandler(store, mockS3)
	h.SetQueue(q)
	h.SetTolerance(10)

	// 3回目の失敗
	body := `{"x": 100, "y": 100}`
	tc := testutil.NewTestContext(http.MethodPost, "/api/captcha/verify", strings.NewReader(body))
	tc.Request.Header.Set("Content-Type", "application/json")
	tc.Request.AddCookie(&http.Cookie{Name: "session_id", Value: sessionID})

	err := h.Verify(tc.Context)
	require.NoError(t, err)

	var resp map[string]interface{}
	_ = json.Unmarshal(tc.Recorder.Body.Bytes(), &resp)

	assert.Equal(t, true, resp["error"])
	assert.Equal(t, float64(3), resp["redirect_delay"])

	// WebSocket接続が閉じられることを確認
	err = testutil.WaitFor(100*time.Millisecond, 10*time.Millisecond, func() bool {
		return mockConn.GetIsClosed()
	})
	require.NoError(t, err)

	// 待機列に追加されたことを確認
	assert.Equal(t, 1, q.Len())
}

func TestCaptchaHandler_Verify_AttemptsRemaining(t *testing.T) {
	tests := []struct {
		name            string
		currentAttempts int
		wantRemaining   int
	}{
		{name: "1回目失敗", currentAttempts: 0, wantRemaining: 2},
		{name: "2回目失敗", currentAttempts: 1, wantRemaining: 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := session.NewSessionStore()
			mockS3 := testutil.NewMockS3Client()
			mockS3.Objects = map[string][]byte{
				"backgrounds/bg1.png":  testutil.CreateTestPNG(2816, 1536),
				"character/char1.png":  testutil.CreateTestPNG(100, 100),
				"character/char2.png":  testutil.CreateTestPNG(100, 100),
				"character/char3.png":  testutil.CreateTestPNG(100, 100),
				"character/char4.png":  testutil.CreateTestPNG(100, 100),
			}

			user, sessionID := store.Create()
			user.Status = "registering"
			user.CaptchaTargetX = 512
			user.CaptchaTargetY = 384
			user.CaptchaAttempts = tt.currentAttempts

			h := NewCaptchaHandler(store, mockS3)
			h.SetTolerance(10)

			body := `{"x": 100, "y": 100}` // 失敗するクリック
			tc := testutil.NewTestContext(http.MethodPost, "/api/captcha/verify", strings.NewReader(body))
			tc.Request.Header.Set("Content-Type", "application/json")
			tc.Request.AddCookie(&http.Cookie{Name: "session_id", Value: sessionID})

			err := h.Verify(tc.Context)
			require.NoError(t, err)

			var resp map[string]interface{}
			_ = json.Unmarshal(tc.Recorder.Body.Bytes(), &resp)

			assert.Equal(t, true, resp["error"])
			assert.Equal(t, float64(tt.wantRemaining), resp["attempts_remaining"])
			assert.NotEmpty(t, resp["new_image_url"]) // 新しい画像URL
		})
	}
}

// itoa converts int to string (simple helper)
func itoa(n int) string {
	return strconv.Itoa(n)
}
