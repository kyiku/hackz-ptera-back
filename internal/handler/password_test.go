package handler

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/kyiku/hackz-ptera-back/internal/model"
	"github.com/kyiku/hackz-ptera-back/internal/session"
	"github.com/kyiku/hackz-ptera-back/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPasswordHandler_Analyze(t *testing.T) {
	tests := []struct {
		name           string
		setupUser      func(*model.User)
		password       string
		mockResponse   string
		mockErr        error
		hasCookie      bool
		wantStatusCode int
		wantError      bool
		wantContains   string
	}{
		{
			name: "正常系: 名前と年を含むパスワード",
			setupUser: func(u *model.User) {
				u.Status = "registering"
			},
			password:       "taro1998",
			mockResponse:   `{"content":[{"text":"太郎さんですか？1998年生まれ？誕生日をパスワードに使うのは危険ですよ！"}]}`,
			hasCookie:      true,
			wantStatusCode: http.StatusOK,
			wantError:      false,
			wantContains:   "太郎",
		},
		{
			name: "正常系: 弱いパスワード",
			setupUser: func(u *model.User) {
				u.Status = "registering"
			},
			password:       "password123",
			mockResponse:   `{"content":[{"text":"これは非常に弱いパスワードです。よく使われるパスワードの上位にランクインしています！"}]}`,
			hasCookie:      true,
			wantStatusCode: http.StatusOK,
			wantError:      false,
			wantContains:   "弱いパスワード",
		},
		{
			name:           "異常系: セッションなし",
			setupUser:      nil,
			password:       "test",
			mockResponse:   "",
			hasCookie:      false,
			wantStatusCode: http.StatusUnauthorized,
			wantError:      true,
			wantContains:   "",
		},
		{
			name: "異常系: waiting状態",
			setupUser: func(u *model.User) {
				u.Status = "waiting"
			},
			password:       "test",
			mockResponse:   "",
			hasCookie:      true,
			wantStatusCode: http.StatusForbidden,
			wantError:      true,
			wantContains:   "",
		},
		{
			name: "異常系: 空のパスワード",
			setupUser: func(u *model.User) {
				u.Status = "registering"
			},
			password:       "",
			mockResponse:   "",
			hasCookie:      true,
			wantStatusCode: http.StatusBadRequest,
			wantError:      true,
			wantContains:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := session.NewSessionStore()
			mockBedrock := testutil.NewMockBedrockClient()
			mockBedrock.Response = tt.mockResponse
			mockBedrock.Err = tt.mockErr

			var sessionID string
			if tt.setupUser != nil {
				user, sid := store.Create()
				tt.setupUser(user)
				sessionID = sid
			}

			h := NewPasswordHandler(store, mockBedrock)

			body := `{"password": "` + tt.password + `"}`
			tc := testutil.NewTestContext(http.MethodPost, "/api/password/analyze", strings.NewReader(body))
			tc.Request.Header.Set("Content-Type", "application/json")
			if tt.hasCookie && sessionID != "" {
				tc.Request.AddCookie(&http.Cookie{Name: "session_id", Value: sessionID})
			}

			err := h.Analyze(tc.Context)

			require.NoError(t, err)
			assert.Equal(t, tt.wantStatusCode, tc.Recorder.Code)

			var resp map[string]interface{}
			json.Unmarshal(tc.Recorder.Body.Bytes(), &resp)

			assert.Equal(t, tt.wantError, resp["error"])

			if tt.wantContains != "" {
				analysis, ok := resp["analysis"].(string)
				assert.True(t, ok)
				assert.Contains(t, analysis, tt.wantContains)
			}
		})
	}
}

func TestPasswordHandler_Analyze_BedrockPrompt(t *testing.T) {
	store := session.NewSessionStore()
	mockBedrock := testutil.NewMockBedrockClient()
	mockBedrock.Response = `{"content":[{"text":"分析結果"}]}`

	user, sessionID := store.Create()
	user.Status = "registering"

	h := NewPasswordHandler(store, mockBedrock)

	body := `{"password": "mySecretPass123"}`
	tc := testutil.NewTestContext(http.MethodPost, "/api/password/analyze", strings.NewReader(body))
	tc.Request.Header.Set("Content-Type", "application/json")
	tc.Request.AddCookie(&http.Cookie{Name: "session_id", Value: sessionID})

	h.Analyze(tc.Context)

	// プロンプトにパスワードが含まれていることを確認
	assert.Contains(t, mockBedrock.LastPrompt, "mySecretPass123")

	// Claude 3 Haikuモデルが使用されていることを確認
	assert.Contains(t, mockBedrock.LastModelID, "claude-3-haiku")
}

func TestPasswordHandler_Analyze_Fallback(t *testing.T) {
	store := session.NewSessionStore()
	mockBedrock := testutil.NewMockBedrockClient()
	mockBedrock.Err = assert.AnError // Bedrockエラーをシミュレート

	user, sessionID := store.Create()
	user.Status = "registering"

	h := NewPasswordHandler(store, mockBedrock)
	h.EnableFallback(true)

	body := `{"password": "test123"}`
	tc := testutil.NewTestContext(http.MethodPost, "/api/password/analyze", strings.NewReader(body))
	tc.Request.Header.Set("Content-Type", "application/json")
	tc.Request.AddCookie(&http.Cookie{Name: "session_id", Value: sessionID})

	err := h.Analyze(tc.Context)

	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, tc.Recorder.Code)

	var resp map[string]interface{}
	json.Unmarshal(tc.Recorder.Body.Bytes(), &resp)

	// フォールバックメッセージが返される
	assert.False(t, resp["error"].(bool))
	assert.NotEmpty(t, resp["analysis"])
}
