package handler

import (
	"encoding/json"
	"net/http"
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

func TestOTPHandler_Send(t *testing.T) {
	tests := []struct {
		name           string
		setupUser      func(*model.User)
		hasCookie      bool
		wantStatusCode int
		wantError      bool
		wantImageURL   bool
		wantFishSet    bool
	}{
		{
			name: "正常系: OTP画像送信成功",
			setupUser: func(u *model.User) {
				u.Status = "registering"
			},
			hasCookie:      true,
			wantStatusCode: http.StatusOK,
			wantError:      false,
			wantImageURL:   true,
			wantFishSet:    true,
		},
		{
			name:           "異常系: セッションなし",
			setupUser:      nil,
			hasCookie:      false,
			wantStatusCode: http.StatusUnauthorized,
			wantError:      true,
			wantImageURL:   false,
			wantFishSet:    false,
		},
		{
			name: "異常系: waiting状態",
			setupUser: func(u *model.User) {
				u.Status = "waiting"
			},
			hasCookie:      true,
			wantStatusCode: http.StatusForbidden,
			wantError:      true,
			wantImageURL:   false,
			wantFishSet:    false,
		},
		{
			name: "異常系: stage1_dino状態",
			setupUser: func(u *model.User) {
				u.Status = "stage1_dino"
			},
			hasCookie:      true,
			wantStatusCode: http.StatusForbidden,
			wantError:      true,
			wantImageURL:   false,
			wantFishSet:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := session.NewSessionStore()
			mockS3 := testutil.NewMockS3Client()
			mockS3.FishImages = []string{"onikamasu", "houhou", "matsukasauo"}

			var sessionID string
			var user *model.User
			if tt.setupUser != nil {
				user, sessionID = store.Create()
				tt.setupUser(user)
			}

			h := NewOTPHandler(store, mockS3)

			tc := testutil.NewTestContext(http.MethodPost, "/api/otp/send", nil)
			if tt.hasCookie && sessionID != "" {
				tc.Request.AddCookie(&http.Cookie{Name: "session_id", Value: sessionID})
			}

			err := h.Send(tc.Context)

			require.NoError(t, err)
			assert.Equal(t, tt.wantStatusCode, tc.Recorder.Code)

			var resp map[string]interface{}
			json.Unmarshal(tc.Recorder.Body.Bytes(), &resp)

			assert.Equal(t, tt.wantError, resp["error"])

			if tt.wantImageURL {
				imageURL, ok := resp["imageUrl"].(string)
				assert.True(t, ok, "imageUrlが存在するべき")
				assert.Contains(t, imageURL, "cloudfront.net/fish/")
				assert.NotEmpty(t, resp["message"])
			}

			if tt.wantFishSet && user != nil {
				assert.NotEmpty(t, user.OTPFishName, "正解の魚名が保存されているべき")
				assert.Equal(t, 0, user.OTPAttempts, "試行回数がリセットされているべき")
			}
		})
	}
}

func TestOTPHandler_Verify_Success(t *testing.T) {
	tests := []struct {
		name       string
		fishName   string
		answer     string
		wantStatus int
		wantError  bool
	}{
		{
			name:       "正常系: カタカナで正解",
			fishName:   "オニカマス",
			answer:     "オニカマス",
			wantStatus: http.StatusOK,
			wantError:  false,
		},
		{
			name:       "正常系: ひらがなで正解",
			fishName:   "オニカマス",
			answer:     "おにかます",
			wantStatus: http.StatusOK,
			wantError:  false,
		},
		{
			name:       "正常系: 混在で正解（カタカナ正解にひらがな回答）",
			fishName:   "ホウボウ",
			answer:     "ほうぼう",
			wantStatus: http.StatusOK,
			wantError:  false,
		},
		{
			name:       "正常系: ひらがな正解にカタカナ回答",
			fishName:   "まつかさうお",
			answer:     "マツカサウオ",
			wantStatus: http.StatusOK,
			wantError:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := session.NewSessionStore()
			mockS3 := testutil.NewMockS3Client()

			user, sessionID := store.Create()
			user.Status = "registering"
			user.OTPFishName = tt.fishName
			user.OTPAttempts = 0

			h := NewOTPHandler(store, mockS3)

			body := `{"answer": "` + tt.answer + `"}`
			tc := testutil.NewTestContext(http.MethodPost, "/api/otp/verify", strings.NewReader(body))
			tc.Request.Header.Set("Content-Type", "application/json")
			tc.Request.AddCookie(&http.Cookie{Name: "session_id", Value: sessionID})

			err := h.Verify(tc.Context)

			require.NoError(t, err)
			assert.Equal(t, tt.wantStatus, tc.Recorder.Code)

			var resp map[string]interface{}
			json.Unmarshal(tc.Recorder.Body.Bytes(), &resp)

			assert.Equal(t, tt.wantError, resp["error"])
		})
	}
}

func TestOTPHandler_Verify_Failure(t *testing.T) {
	tests := []struct {
		name            string
		fishName        string
		answer          string
		currentAttempts int
		wantStatus      int
		wantRemaining   int
		wantNewImage    bool
		wantRedirect    bool
		wantConnClosed  bool
		wantQueueReset  bool
	}{
		{
			name:            "失敗1回目: 残り2回",
			fishName:        "オニカマス",
			answer:          "wrong",
			currentAttempts: 0,
			wantStatus:      http.StatusOK,
			wantRemaining:   2,
			wantNewImage:    true,
			wantRedirect:    false,
			wantConnClosed:  false,
			wantQueueReset:  false,
		},
		{
			name:            "失敗2回目: 残り1回",
			fishName:        "オニカマス",
			answer:          "wrong",
			currentAttempts: 1,
			wantStatus:      http.StatusOK,
			wantRemaining:   1,
			wantNewImage:    true,
			wantRedirect:    false,
			wantConnClosed:  false,
			wantQueueReset:  false,
		},
		{
			name:            "失敗3回目: 最後尾へリセット",
			fishName:        "オニカマス",
			answer:          "wrong",
			currentAttempts: 2,
			wantStatus:      http.StatusOK,
			wantRemaining:   0,
			wantNewImage:    false,
			wantRedirect:    true,
			wantConnClosed:  true,
			wantQueueReset:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := session.NewSessionStore()
			mockS3 := testutil.NewMockS3Client()
			mockS3.FishImages = []string{"newfish1", "newfish2"}
			q := queue.NewWaitingQueue()

			mockConn := testutil.NewMockWebSocketConn()
			user, sessionID := store.Create()
			user.Status = "registering"
			user.OTPFishName = tt.fishName
			user.OTPAttempts = tt.currentAttempts
			user.Conn = mockConn

			h := NewOTPHandler(store, mockS3)
			h.SetQueue(q)

			body := `{"answer": "` + tt.answer + `"}`
			tc := testutil.NewTestContext(http.MethodPost, "/api/otp/verify", strings.NewReader(body))
			tc.Request.Header.Set("Content-Type", "application/json")
			tc.Request.AddCookie(&http.Cookie{Name: "session_id", Value: sessionID})

			err := h.Verify(tc.Context)

			require.NoError(t, err)
			assert.Equal(t, tt.wantStatus, tc.Recorder.Code)

			var resp map[string]interface{}
			json.Unmarshal(tc.Recorder.Body.Bytes(), &resp)

			assert.True(t, resp["error"].(bool))

			if tt.wantNewImage {
				remaining := int(resp["attemptsRemaining"].(float64))
				assert.Equal(t, tt.wantRemaining, remaining)
				assert.NotEmpty(t, resp["newImageUrl"])
			}

			if tt.wantRedirect {
				assert.Equal(t, float64(3), resp["redirectDelay"])
			}

			if tt.wantConnClosed {
				// WaitForで接続が閉じられることを確認
				err := testutil.WaitFor(100*time.Millisecond, 10*time.Millisecond, func() bool {
					return mockConn.IsClosed
				})
				require.NoError(t, err, "WebSocket接続が閉じられるべき")
			}

			if tt.wantQueueReset {
				assert.Equal(t, 1, q.Len(), "待機列に追加されるべき")
				assert.Equal(t, "waiting", user.Status)
			}
		})
	}
}
