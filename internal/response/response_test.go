package response

import (
	"encoding/json"
	"net/http"
	"testing"

	"hackz-ptera/back/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResponse_Success(t *testing.T) {
	tests := []struct {
		name       string
		data       map[string]interface{}
		wantFields []string
	}{
		{
			name: "正常系: 基本的な成功レスポンス",
			data: map[string]interface{}{
				"token":   "abc123",
				"message": "成功しました",
			},
			wantFields: []string{"error", "token", "message"},
		},
		{
			name:       "正常系: 空のデータ",
			data:       map[string]interface{}{},
			wantFields: []string{"error"},
		},
		{
			name: "正常系: ネストしたデータ",
			data: map[string]interface{}{
				"user": map[string]interface{}{
					"id":   "123",
					"name": "test",
				},
			},
			wantFields: []string{"error", "user"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tc := testutil.NewTestContext(http.MethodGet, "/", nil)

			err := Success(tc.Context, tt.data)

			require.NoError(t, err)
			assert.Equal(t, http.StatusOK, tc.Recorder.Code)

			var resp map[string]interface{}
			json.Unmarshal(tc.Recorder.Body.Bytes(), &resp)

			assert.Equal(t, false, resp["error"])
			for _, field := range tt.wantFields {
				assert.Contains(t, resp, field)
			}
		})
	}
}

func TestResponse_Error(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		message    string
		wantStatus int
		wantError  bool
	}{
		{
			name:       "BadRequest",
			statusCode: http.StatusBadRequest,
			message:    "入力が不正です",
			wantStatus: http.StatusBadRequest,
			wantError:  true,
		},
		{
			name:       "Unauthorized",
			statusCode: http.StatusUnauthorized,
			message:    "認証が必要です",
			wantStatus: http.StatusUnauthorized,
			wantError:  true,
		},
		{
			name:       "Forbidden",
			statusCode: http.StatusForbidden,
			message:    "アクセスが拒否されました",
			wantStatus: http.StatusForbidden,
			wantError:  true,
		},
		{
			name:       "InternalServerError",
			statusCode: http.StatusInternalServerError,
			message:    "サーバーエラーが発生しました",
			wantStatus: http.StatusInternalServerError,
			wantError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tc := testutil.NewTestContext(http.MethodGet, "/", nil)

			err := Error(tc.Context, tt.statusCode, tt.message)

			require.NoError(t, err)
			assert.Equal(t, tt.wantStatus, tc.Recorder.Code)

			var resp map[string]interface{}
			json.Unmarshal(tc.Recorder.Body.Bytes(), &resp)

			assert.Equal(t, tt.wantError, resp["error"])
			assert.Equal(t, tt.message, resp["message"])
			assert.Nil(t, resp["redirect_delay"], "通常のエラーにはredirect_delayがないべき")
		})
	}
}

func TestResponse_ErrorWithRedirect(t *testing.T) {
	tests := []struct {
		name          string
		message       string
		redirectDelay int
		wantStatus    int
		wantDelay     float64
	}{
		{
			name:          "正常系: 3秒リダイレクト",
			message:       "3回失敗しました。",
			redirectDelay: 3,
			wantStatus:    http.StatusOK,
			wantDelay:     3,
		},
		{
			name:          "正常系: 5秒リダイレクト",
			message:       "タイムアウトしました。",
			redirectDelay: 5,
			wantStatus:    http.StatusOK,
			wantDelay:     5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tc := testutil.NewTestContext(http.MethodGet, "/", nil)

			err := ErrorWithRedirect(tc.Context, tt.message, tt.redirectDelay)

			require.NoError(t, err)
			assert.Equal(t, tt.wantStatus, tc.Recorder.Code)

			var resp map[string]interface{}
			json.Unmarshal(tc.Recorder.Body.Bytes(), &resp)

			assert.Equal(t, true, resp["error"])
			assert.Equal(t, tt.message, resp["message"])
			assert.Equal(t, tt.wantDelay, resp["redirect_delay"])
		})
	}
}

func TestResponse_ErrorWithCode(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		code       string
		message    string
		wantCode   string
	}{
		{
			name:       "SESSION_EXPIRED",
			statusCode: http.StatusUnauthorized,
			code:       "SESSION_EXPIRED",
			message:    "セッションが期限切れです",
			wantCode:   "SESSION_EXPIRED",
		},
		{
			name:       "TOKEN_EXPIRED",
			statusCode: http.StatusUnauthorized,
			code:       "TOKEN_EXPIRED",
			message:    "トークンが期限切れです",
			wantCode:   "TOKEN_EXPIRED",
		},
		{
			name:       "INVALID_SESSION",
			statusCode: http.StatusUnauthorized,
			code:       "INVALID_SESSION",
			message:    "無効なセッションです",
			wantCode:   "INVALID_SESSION",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tc := testutil.NewTestContext(http.MethodGet, "/", nil)

			err := ErrorWithCode(tc.Context, tt.statusCode, tt.code, tt.message)

			require.NoError(t, err)
			assert.Equal(t, tt.statusCode, tc.Recorder.Code)

			var resp map[string]interface{}
			json.Unmarshal(tc.Recorder.Body.Bytes(), &resp)

			assert.Equal(t, true, resp["error"])
			assert.Equal(t, tt.wantCode, resp["code"])
			assert.Equal(t, tt.message, resp["message"])
		})
	}
}

func TestResponse_ContentType(t *testing.T) {
	tc := testutil.NewTestContext(http.MethodGet, "/", nil)

	Success(tc.Context, map[string]interface{}{"test": true})

	contentType := tc.Recorder.Header().Get("Content-Type")
	assert.Contains(t, contentType, "application/json")
}
