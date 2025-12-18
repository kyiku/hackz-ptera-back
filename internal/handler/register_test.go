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

func TestRegisterHandler_Submit(t *testing.T) {
	tests := []struct {
		name           string
		setupUser      func(*model.User)
		requestBody    string
		hasCookie      bool
		wantStatusCode int
		wantError      bool
		wantRedirect   bool
		wantConnClosed bool
	}{
		{
			name: "鬼畜仕様: 常にサーバーエラーで失敗",
			setupUser: func(u *model.User) {
				u.Status = "registering"
				u.RegisterToken = "valid-token"
				u.RegisterTokenExp = time.Now().Add(10 * time.Minute)
			},
			requestBody: `{
				"username": "testuser",
				"email": "test@example.com",
				"password": "password123",
				"token": "valid-token"
			}`,
			hasCookie:      true,
			wantStatusCode: http.StatusInternalServerError,
			wantError:      true,
			wantRedirect:   true,
			wantConnClosed: true,
		},
		{
			name:           "異常系: セッションなし",
			setupUser:      nil,
			requestBody:    `{"username": "test"}`,
			hasCookie:      false,
			wantStatusCode: http.StatusUnauthorized,
			wantError:      true,
			wantRedirect:   false,
			wantConnClosed: false,
		},
		{
			name: "異常系: waiting状態",
			setupUser: func(u *model.User) {
				u.Status = "waiting"
			},
			requestBody:    `{"username": "test"}`,
			hasCookie:      true,
			wantStatusCode: http.StatusForbidden,
			wantError:      true,
			wantRedirect:   false,
			wantConnClosed: false,
		},
		{
			name: "異常系: 不正なJSON",
			setupUser: func(u *model.User) {
				u.Status = "registering"
			},
			requestBody:    `{invalid}`,
			hasCookie:      true,
			wantStatusCode: http.StatusBadRequest,
			wantError:      true,
			wantRedirect:   false,
			wantConnClosed: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := session.NewSessionStore()
			q := queue.NewWaitingQueue()

			var sessionID string
			var user *model.User
			var mockConn *testutil.MockWebSocketConn

			if tt.setupUser != nil {
				mockConn = testutil.NewMockWebSocketConn()
				user, sessionID = store.Create()
				tt.setupUser(user)
				user.Conn = mockConn
			}

			h := NewRegisterHandler(store)
			h.SetQueue(q)

			tc := testutil.NewTestContext(http.MethodPost, "/api/register", strings.NewReader(tt.requestBody))
			tc.Request.Header.Set("Content-Type", "application/json")
			if tt.hasCookie && sessionID != "" {
				tc.Request.AddCookie(&http.Cookie{Name: "session_id", Value: sessionID})
			}

			err := h.Submit(tc.Context)

			require.NoError(t, err)
			assert.Equal(t, tt.wantStatusCode, tc.Recorder.Code)

			var resp map[string]interface{}
			json.Unmarshal(tc.Recorder.Body.Bytes(), &resp)

			assert.Equal(t, tt.wantError, resp["error"])

			if tt.wantRedirect {
				assert.Equal(t, float64(3), resp["redirectDelay"])
			}

			if tt.wantConnClosed && mockConn != nil {
				err := testutil.WaitFor(100*time.Millisecond, 10*time.Millisecond, func() bool {
					return mockConn.IsClosed
				})
				require.NoError(t, err, "WebSocket接続が閉じられるべき")
			}
		})
	}
}

func TestRegisterHandler_AlwaysFails(t *testing.T) {
	// 鬼畜仕様: 登録は絶対に成功しない
	store := session.NewSessionStore()
	q := queue.NewWaitingQueue()

	mockConn := testutil.NewMockWebSocketConn()
	user, sessionID := store.Create()
	user.Status = "registering"
	user.RegisterToken = "valid-token"
	user.RegisterTokenExp = time.Now().Add(10 * time.Minute)
	user.Conn = mockConn

	h := NewRegisterHandler(store)
	h.SetQueue(q)

	// 何度試しても失敗する
	for i := 0; i < 5; i++ {
		// 新しいユーザーを作成（前回は待機列に戻されるため）
		if i > 0 {
			mockConn = testutil.NewMockWebSocketConn()
			user, sessionID = store.Create()
			user.Status = "registering"
			user.RegisterToken = "valid-token"
			user.RegisterTokenExp = time.Now().Add(10 * time.Minute)
			user.Conn = mockConn
		}

		body := `{
			"username": "testuser",
			"email": "test@example.com",
			"password": "StrongP@ssw0rd!",
			"token": "valid-token"
		}`
		tc := testutil.NewTestContext(http.MethodPost, "/api/register", strings.NewReader(body))
		tc.Request.Header.Set("Content-Type", "application/json")
		tc.Request.AddCookie(&http.Cookie{Name: "session_id", Value: sessionID})

		err := h.Submit(tc.Context)
		require.NoError(t, err)

		// 常にエラー
		assert.Equal(t, http.StatusInternalServerError, tc.Recorder.Code)

		var resp map[string]interface{}
		json.Unmarshal(tc.Recorder.Body.Bytes(), &resp)

		assert.True(t, resp["error"].(bool))
		assert.Contains(t, resp["message"], "サーバーエラー")
	}
}

func TestRegisterHandler_QueueReset(t *testing.T) {
	store := session.NewSessionStore()
	q := queue.NewWaitingQueue()

	mockConn := testutil.NewMockWebSocketConn()
	user, sessionID := store.Create()
	user.Status = "registering"
	user.RegisterToken = "valid-token"
	user.RegisterTokenExp = time.Now().Add(10 * time.Minute)
	user.Conn = mockConn

	h := NewRegisterHandler(store)
	h.SetQueue(q)

	body := `{
		"username": "testuser",
		"email": "test@example.com",
		"password": "password123",
		"token": "valid-token"
	}`
	tc := testutil.NewTestContext(http.MethodPost, "/api/register", strings.NewReader(body))
	tc.Request.Header.Set("Content-Type", "application/json")
	tc.Request.AddCookie(&http.Cookie{Name: "session_id", Value: sessionID})

	h.Submit(tc.Context)

	// 待機列に追加されたことを確認
	err := testutil.WaitFor(100*time.Millisecond, 10*time.Millisecond, func() bool {
		return q.Len() == 1
	})
	require.NoError(t, err, "待機列に追加されるべき")

	// ステータスがwaitingに戻っていることを確認
	assert.Equal(t, "waiting", user.Status)
}

func TestRegisterHandler_WebSocketFailureMessage(t *testing.T) {
	store := session.NewSessionStore()
	q := queue.NewWaitingQueue()

	mockConn := testutil.NewMockWebSocketConn()
	user, sessionID := store.Create()
	user.Status = "registering"
	user.RegisterToken = "valid-token"
	user.RegisterTokenExp = time.Now().Add(10 * time.Minute)
	user.Conn = mockConn

	h := NewRegisterHandler(store)
	h.SetQueue(q)

	body := `{
		"username": "testuser",
		"email": "test@example.com",
		"password": "password123",
		"token": "valid-token"
	}`
	tc := testutil.NewTestContext(http.MethodPost, "/api/register", strings.NewReader(body))
	tc.Request.Header.Set("Content-Type", "application/json")
	tc.Request.AddCookie(&http.Cookie{Name: "session_id", Value: sessionID})

	h.Submit(tc.Context)

	// WebSocket経由でfailureメッセージが送信されることを確認
	msg := testutil.WaitForMessage(mockConn, 100*time.Millisecond)
	require.NotNil(t, msg, "WebSocketメッセージが送信されるべき")

	assert.Equal(t, "failure", msg["type"])
	assert.Equal(t, float64(3), msg["redirectDelay"])
}
