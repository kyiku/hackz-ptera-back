package token

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"hackz-ptera/back/internal/model"
	"hackz-ptera/back/internal/queue"
	"hackz-ptera/back/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegisterToken_Generate(t *testing.T) {
	tests := []struct {
		name         string
		userID       string
		sessionID    string
		wantTokenSet bool
		wantExpAfter time.Duration
	}{
		{
			name:         "正常系: トークン生成",
			userID:       "user1",
			sessionID:    "session1",
			wantTokenSet: true,
			wantExpAfter: 9 * time.Minute, // 10分の有効期限、少し余裕をもって検証
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user := &model.User{ID: tt.userID, SessionID: tt.sessionID}

			token := GenerateRegisterToken(user)

			// トークンが空でないことを確認
			assert.NotEmpty(t, token)

			// User構造体に保存されていることを確認
			assert.Equal(t, token, user.RegisterToken)

			// UUID形式の確認
			_, err := uuid.Parse(token)
			assert.NoError(t, err, "UUID形式であるべき")

			// 有効期限が設定されていることを確認
			assert.False(t, user.RegisterTokenExp.IsZero())
			assert.True(t, user.RegisterTokenExp.After(time.Now().Add(tt.wantExpAfter)))
		})
	}
}

func TestRegisterToken_Validate(t *testing.T) {
	// 有効なユーザーとトークンを作成
	user := &model.User{ID: "user1", SessionID: "session1"}
	validToken := GenerateRegisterToken(user)

	tests := []struct {
		name      string
		sessionID string
		token     string
		wantValid bool
		wantError string
	}{
		{
			name:      "正常系: 有効なトークン",
			sessionID: "session1",
			token:     validToken,
			wantValid: true,
			wantError: "",
		},
		{
			name:      "異常系: セッションIDが異なる",
			sessionID: "wrong_session",
			token:     validToken,
			wantValid: false,
			wantError: "INVALID_SESSION",
		},
		{
			name:      "異常系: トークンが異なる",
			sessionID: "session1",
			token:     "invalid-token-uuid",
			wantValid: false,
			wantError: "INVALID_TOKEN",
		},
		{
			name:      "異常系: 空のトークン",
			sessionID: "session1",
			token:     "",
			wantValid: false,
			wantError: "INVALID_TOKEN",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			valid, errCode := ValidateRegisterToken(user, tt.sessionID, tt.token)

			assert.Equal(t, tt.wantValid, valid)
			if tt.wantError != "" {
				assert.Equal(t, tt.wantError, errCode)
			}
		})
	}
}

func TestRegisterToken_Expired(t *testing.T) {
	tests := []struct {
		name        string
		expOffset   time.Duration
		wantExpired bool
	}{
		{
			name:        "正常系: 有効期限内",
			expOffset:   10 * time.Minute,
			wantExpired: false,
		},
		{
			name:        "異常系: 有効期限切れ",
			expOffset:   -1 * time.Minute,
			wantExpired: true,
		},
		{
			name:        "境界値: ちょうど期限切れ",
			expOffset:   0,
			wantExpired: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user := &model.User{
				ID:               "user1",
				SessionID:        "session1",
				RegisterToken:    "test-token",
				RegisterTokenExp: time.Now().Add(tt.expOffset),
			}

			expired := IsTokenExpired(user)
			assert.Equal(t, tt.wantExpired, expired)
		})
	}
}

func TestRegisterToken_Monitor(t *testing.T) {
	tests := []struct {
		name           string
		tokenExp       time.Duration
		checkInterval  time.Duration
		wantExpired    bool
		wantConnClosed bool
		wantQueueReset bool
	}{
		{
			name:           "正常系: 期限切れでWebSocket切断・待機列リセット",
			tokenExp:       50 * time.Millisecond,
			checkInterval:  10 * time.Millisecond,
			wantExpired:    true,
			wantConnClosed: true,
			wantQueueReset: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockConn := testutil.NewMockWebSocketConn()
			q := queue.NewWaitingQueue()

			user := &model.User{
				ID:               "user1",
				SessionID:        "session1",
				Status:           "registering",
				RegisterToken:    "test-token",
				RegisterTokenExp: time.Now().Add(tt.tokenExp),
				Conn:             mockConn,
			}

			monitor := NewTokenMonitor(tt.checkInterval)
			monitor.SetQueue(q)
			monitor.Watch(user)
			defer monitor.Stop()

			// WaitForで期限切れ処理完了を待機
			err := testutil.WaitFor(200*time.Millisecond, 10*time.Millisecond, func() bool {
				return mockConn.IsClosed
			})
			require.NoError(t, err, "トークン期限切れ処理が完了しなかった")

			// WebSocket通知を確認
			msg := testutil.WaitForMessage(mockConn, 100*time.Millisecond)
			require.NotNil(t, msg, "メッセージが受信されなかった")
			assert.Equal(t, "TOKEN_EXPIRED", msg["code"])

			// 接続が閉じられたことを確認
			assert.Equal(t, tt.wantConnClosed, mockConn.IsClosed)

			// 待機列に追加されたことを確認
			if tt.wantQueueReset {
				assert.Equal(t, 1, q.Len())
			}
		})
	}
}

func TestRegisterToken_MonitorCancel(t *testing.T) {
	mockConn := testutil.NewMockWebSocketConn()

	user := &model.User{
		ID:               "user1",
		SessionID:        "session1",
		Status:           "registering",
		RegisterToken:    "test-token",
		RegisterTokenExp: time.Now().Add(100 * time.Millisecond),
		Conn:             mockConn,
	}

	monitor := NewTokenMonitor(10 * time.Millisecond)
	monitor.Watch(user)

	// 期限切れ前に監視をキャンセル
	monitor.Unwatch(user)

	// 期限切れ時間が過ぎても接続が閉じられないことを確認
	err := testutil.WaitFor(200*time.Millisecond, 10*time.Millisecond, func() bool {
		return mockConn.IsClosed
	})
	// エラーになることを期待（接続が閉じられていない）
	assert.Error(t, err, "監視キャンセル後は接続が閉じられないべき")
	assert.False(t, mockConn.IsClosed)
}
