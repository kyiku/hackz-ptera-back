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

func TestDinoHandler_Result(t *testing.T) {
	tests := []struct {
		name           string
		setupUser      func(*model.User)
		requestBody    string
		hasCookie      bool
		wantStatusCode int
		wantError      bool
		wantNextStage  string
	}{
		{
			name: "正常系: ゲームクリア",
			setupUser: func(u *model.User) {
				u.Status = "stage1_dino"
			},
			requestBody:    `{"result": "clear", "score": 1000}`,
			hasCookie:      true,
			wantStatusCode: http.StatusOK,
			wantError:      false,
			wantNextStage:  "register",
		},
		{
			name: "正常系: ゲームオーバー",
			setupUser: func(u *model.User) {
				u.Status = "stage1_dino"
			},
			requestBody:    `{"result": "gameover", "score": 500}`,
			hasCookie:      true,
			wantStatusCode: http.StatusOK,
			wantError:      true,
			wantNextStage:  "",
		},
		{
			name:           "異常系: セッションなし",
			setupUser:      nil,
			requestBody:    `{"result": "clear", "score": 1000}`,
			hasCookie:      false,
			wantStatusCode: http.StatusUnauthorized,
			wantError:      true,
			wantNextStage:  "",
		},
		{
			name: "異常系: 不正なステータス（waiting）",
			setupUser: func(u *model.User) {
				u.Status = "waiting"
			},
			requestBody:    `{"result": "clear", "score": 1000}`,
			hasCookie:      true,
			wantStatusCode: http.StatusForbidden,
			wantError:      true,
			wantNextStage:  "",
		},
		{
			name: "異常系: 不正なステータス（registering）",
			setupUser: func(u *model.User) {
				u.Status = "registering"
			},
			requestBody:    `{"result": "clear", "score": 1000}`,
			hasCookie:      true,
			wantStatusCode: http.StatusForbidden,
			wantError:      true,
			wantNextStage:  "",
		},
		{
			name: "異常系: 不正なリクエストボディ",
			setupUser: func(u *model.User) {
				u.Status = "stage1_dino"
			},
			requestBody:    `{invalid json}`,
			hasCookie:      true,
			wantStatusCode: http.StatusBadRequest,
			wantError:      true,
			wantNextStage:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := session.NewSessionStore()

			var sessionID string
			var user *model.User
			if tt.setupUser != nil {
				user, sessionID = store.Create()
				tt.setupUser(user)
			}

			h := NewDinoHandler(store)

			tc := testutil.NewTestContext(http.MethodPost, "/api/game/dino/result", strings.NewReader(tt.requestBody))
			tc.Request.Header.Set("Content-Type", "application/json")
			if tt.hasCookie && sessionID != "" {
				tc.Request.AddCookie(&http.Cookie{Name: "session_id", Value: sessionID})
			}

			err := h.Result(tc.Context)

			require.NoError(t, err)
			assert.Equal(t, tt.wantStatusCode, tc.Recorder.Code)

			var resp map[string]interface{}
			json.Unmarshal(tc.Recorder.Body.Bytes(), &resp)

			assert.Equal(t, tt.wantError, resp["error"])

			if tt.wantNextStage != "" {
				assert.Equal(t, tt.wantNextStage, resp["nextStage"])
			}
		})
	}
}

func TestDinoHandler_Result_GameOver_QueueReset(t *testing.T) {
	store := session.NewSessionStore()
	user, sessionID := store.Create()
	user.Status = "stage1_dino"
	mockConn := testutil.NewMockWebSocketConn()
	user.Conn = mockConn

	h := NewDinoHandler(store)

	tc := testutil.NewTestContext(http.MethodPost, "/api/game/dino/result", strings.NewReader(`{"result": "gameover", "score": 100}`))
	tc.Request.Header.Set("Content-Type", "application/json")
	tc.Request.AddCookie(&http.Cookie{Name: "session_id", Value: sessionID})

	err := h.Result(tc.Context)

	require.NoError(t, err)

	var resp map[string]interface{}
	json.Unmarshal(tc.Recorder.Body.Bytes(), &resp)

	assert.Equal(t, true, resp["error"])
	assert.Equal(t, float64(3), resp["redirectDelay"])
}
