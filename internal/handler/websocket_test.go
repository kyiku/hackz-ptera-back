package handler

import (
	"net/http"
	"testing"
	"time"

	"github.com/kyiku/hackz-ptera-back/internal/queue"
	"github.com/kyiku/hackz-ptera-back/internal/session"
	"github.com/kyiku/hackz-ptera-back/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWebSocketHandler_Connect(t *testing.T) {
	tests := []struct {
		name           string
		hasCookie      bool
		validSession   bool
		wantStatusCode int
		wantInQueue    bool
	}{
		{
			name:           "正常系: 有効なセッションで接続",
			hasCookie:      true,
			validSession:   true,
			wantStatusCode: http.StatusSwitchingProtocols,
			wantInQueue:    true,
		},
		{
			name:           "異常系: セッションなし",
			hasCookie:      false,
			validSession:   false,
			wantStatusCode: http.StatusUnauthorized,
			wantInQueue:    false,
		},
		{
			name:           "異常系: 無効なセッション",
			hasCookie:      true,
			validSession:   false,
			wantStatusCode: http.StatusUnauthorized,
			wantInQueue:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := session.NewSessionStore()
			q := queue.NewWaitingQueue()

			var sessionID string
			if tt.validSession {
				_, sessionID = store.Create()
			}

			tc := testutil.NewTestContext(http.MethodGet, "/ws", nil)
			if tt.hasCookie {
				if tt.validSession {
					tc.Request.AddCookie(&http.Cookie{Name: "session_id", Value: sessionID})
				} else {
					tc.Request.AddCookie(&http.Cookie{Name: "session_id", Value: "invalid-session"})
				}
			}

			h := NewWebSocketHandler(store, q)

			// Note: 実際のWebSocketアップグレードはhttptestではテストできないため、
			// セッション検証のロジックのみテスト
			err := h.ValidateSession(tc.Context)

			if tt.validSession && tt.hasCookie {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
		})
	}
}

func TestWebSocketHandler_QueueUpdate(t *testing.T) {
	tests := []struct {
		name          string
		usersInQueue  int
		userPosition  int
		wantPosition  int
		wantTotal     int
	}{
		{
			name:         "正常系: 先頭ユーザー",
			usersInQueue: 5,
			userPosition: 0,
			wantPosition: 1,
			wantTotal:    5,
		},
		{
			name:         "正常系: 中間ユーザー",
			usersInQueue: 10,
			userPosition: 4,
			wantPosition: 5,
			wantTotal:    10,
		},
		{
			name:         "正常系: 最後尾ユーザー",
			usersInQueue: 3,
			userPosition: 2,
			wantPosition: 3,
			wantTotal:    3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			q := queue.NewWaitingQueue()
			mockConns := make([]*testutil.MockWebSocketConn, tt.usersInQueue)

			// ユーザーを追加
			for i := 0; i < tt.usersInQueue; i++ {
				mockConns[i] = testutil.NewMockWebSocketConn()
				q.Add(&queue.QueueUser{
					ID:   string(rune('a' + i)),
					Conn: mockConns[i],
				})
			}

			// キュー更新をブロードキャスト
			q.BroadcastPositions()

			// 対象ユーザーのメッセージを確認
			err := testutil.WaitFor(100*time.Millisecond, 10*time.Millisecond, func() bool {
				return mockConns[tt.userPosition].LastMessage != nil
			})
			require.NoError(t, err)

			msg := mockConns[tt.userPosition].GetLastMessageAsMap()
			require.NotNil(t, msg)

			assert.Equal(t, "queue_update", msg["type"])
			assert.Equal(t, float64(tt.wantPosition), msg["position"])
			assert.Equal(t, float64(tt.wantTotal), msg["total"])
		})
	}
}

func TestWebSocketHandler_Disconnect(t *testing.T) {
	store := session.NewSessionStore()
	q := queue.NewWaitingQueue()

	user, sessionID := store.Create()
	mockConn := testutil.NewMockWebSocketConn()

	q.Add(&queue.QueueUser{
		ID:        user.ID,
		SessionID: sessionID,
		Conn:      mockConn,
	})

	assert.Equal(t, 1, q.Len())

	// 切断をシミュレート
	q.Remove(user.ID)

	assert.Equal(t, 0, q.Len())
}
