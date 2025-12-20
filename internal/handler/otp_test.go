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
		name             string
		setupUser        func(*model.User)
		hasCookie        bool
		wantStatusCode   int
		wantError        bool
		wantProblemLatex bool
		wantOTPSet       bool
	}{
		{
			name: "正常系: 微分問題送信成功",
			setupUser: func(u *model.User) {
				u.Status = "registering"
			},
			hasCookie:        true,
			wantStatusCode:   http.StatusOK,
			wantError:        false,
			wantProblemLatex: true,
			wantOTPSet:       true,
		},
		{
			name:             "異常系: セッションなし",
			setupUser:        nil,
			hasCookie:        false,
			wantStatusCode:   http.StatusOK,
			wantError:        true,
			wantProblemLatex: false,
			wantOTPSet:       false,
		},
		{
			name: "異常系: waiting状態",
			setupUser: func(u *model.User) {
				u.Status = "waiting"
			},
			hasCookie:        true,
			wantStatusCode:   http.StatusOK,
			wantError:        true,
			wantProblemLatex: false,
			wantOTPSet:       false,
		},
		{
			name: "異常系: stage1_dino状態",
			setupUser: func(u *model.User) {
				u.Status = "stage1_dino"
			},
			hasCookie:        true,
			wantStatusCode:   http.StatusOK,
			wantError:        true,
			wantProblemLatex: false,
			wantOTPSet:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := session.NewSessionStore()
			mockS3 := testutil.NewMockS3Client()

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
			_ = json.Unmarshal(tc.Recorder.Body.Bytes(), &resp)

			assert.Equal(t, tt.wantError, resp["error"])

			if tt.wantProblemLatex {
				problemLatex, ok := resp["problem_latex"].(string)
				assert.True(t, ok, "problem_latexが存在するべき")
				assert.Contains(t, problemLatex, "x^2")
				assert.Contains(t, problemLatex, "微分")
				assert.NotEmpty(t, resp["message"])
			}

			if tt.wantOTPSet && user != nil {
				assert.NotZero(t, user.OTPCode, "OTPが保存されているべき")
				assert.GreaterOrEqual(t, user.OTPCode, 100000, "OTPは6桁以上")
				assert.LessOrEqual(t, user.OTPCode, 999999, "OTPは6桁以下")
				assert.Equal(t, 0, user.OTPAttempts, "試行回数がリセットされているべき")
			}
		})
	}
}

func TestOTPHandler_Verify_Success(t *testing.T) {
	tests := []struct {
		name       string
		otpCode    int
		answer     string
		wantStatus int
		wantError  bool
	}{
		{
			name:       "正常系: 正しい6桁回答",
			otpCode:    123456,
			answer:     "123456",
			wantStatus: http.StatusOK,
			wantError:  false,
		},
		{
			name:       "正常系: 6桁境界値（最小）",
			otpCode:    100000,
			answer:     "100000",
			wantStatus: http.StatusOK,
			wantError:  false,
		},
		{
			name:       "正常系: 6桁境界値（最大）",
			otpCode:    999999,
			answer:     "999999",
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
			user.OTPCode = tt.otpCode
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
			_ = json.Unmarshal(tc.Recorder.Body.Bytes(), &resp)

			assert.Equal(t, tt.wantError, resp["error"])
		})
	}
}

func TestOTPHandler_Verify_Failure(t *testing.T) {
	tests := []struct {
		name             string
		otpCode          int
		answer           string
		currentAttempts  int
		wantStatus       int
		wantRemaining    int
		wantNewProblem   bool
		wantRedirect     bool
		wantConnClosed   bool
		wantStatusReset  bool
	}{
		{
			name:            "失敗1回目: 残り2回",
			otpCode:         123456,
			answer:          "654321",
			currentAttempts: 0,
			wantStatus:      http.StatusOK,
			wantRemaining:   2,
			wantNewProblem:  true,
			wantRedirect:    false,
			wantConnClosed:  false,
			wantStatusReset: false,
		},
		{
			name:            "失敗2回目: 残り1回",
			otpCode:         123456,
			answer:          "111111",
			currentAttempts: 1,
			wantStatus:      http.StatusOK,
			wantRemaining:   1,
			wantNewProblem:  true,
			wantRedirect:    false,
			wantConnClosed:  false,
			wantStatusReset: false,
		},
		{
			name:            "失敗3回目: 最後尾へリセット",
			otpCode:         123456,
			answer:          "000000",
			currentAttempts: 2,
			wantStatus:      http.StatusOK,
			wantRemaining:   0,
			wantNewProblem:  false,
			wantRedirect:    true,
			wantConnClosed:  true,
			wantStatusReset: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := session.NewSessionStore()
			mockS3 := testutil.NewMockS3Client()
			q := queue.NewWaitingQueue()

			mockConn := testutil.NewMockWebSocketConn()
			user, sessionID := store.Create()
			user.Status = "registering"
			user.OTPCode = tt.otpCode
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
			_ = json.Unmarshal(tc.Recorder.Body.Bytes(), &resp)

			assert.True(t, resp["error"].(bool))

			if tt.wantNewProblem {
				remaining := int(resp["attempts_remaining"].(float64))
				assert.Equal(t, tt.wantRemaining, remaining)
				assert.NotEmpty(t, resp["new_problem_latex"])
			}

			if tt.wantRedirect {
				assert.Equal(t, float64(3), resp["redirect_delay"])
			}

			if tt.wantConnClosed {
				// WaitForで接続が閉じられることを確認
				err := testutil.WaitFor(100*time.Millisecond, 10*time.Millisecond, func() bool {
					return mockConn.GetIsClosed()
				})
				require.NoError(t, err, "WebSocket接続が閉じられるべき")
			}

			if tt.wantStatusReset {
				// 待機列には追加されない（再接続時に追加）
				assert.Equal(t, 0, q.Len())
				assert.Equal(t, "waiting", user.Status)
			}
		})
	}
}

func TestOTPHandler_Verify_InvalidAnswer(t *testing.T) {
	tests := []struct {
		name      string
		answer    string
		wantError bool
		wantCode  string
	}{
		{
			name:      "異常系: 非数値",
			answer:    "abcdef",
			wantError: true,
			wantCode:  "INVALID_ANSWER",
		},
		{
			name:      "異常系: 空文字",
			answer:    "",
			wantError: true,
			wantCode:  "INVALID_ANSWER",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := session.NewSessionStore()
			mockS3 := testutil.NewMockS3Client()

			user, sessionID := store.Create()
			user.Status = "registering"
			user.OTPCode = 123456

			h := NewOTPHandler(store, mockS3)

			body := `{"answer": "` + tt.answer + `"}`
			tc := testutil.NewTestContext(http.MethodPost, "/api/otp/verify", strings.NewReader(body))
			tc.Request.Header.Set("Content-Type", "application/json")
			tc.Request.AddCookie(&http.Cookie{Name: "session_id", Value: sessionID})

			err := h.Verify(tc.Context)

			require.NoError(t, err)

			var resp map[string]interface{}
			_ = json.Unmarshal(tc.Recorder.Body.Bytes(), &resp)

			assert.Equal(t, tt.wantError, resp["error"])
			assert.Equal(t, tt.wantCode, resp["code"])
		})
	}
}
