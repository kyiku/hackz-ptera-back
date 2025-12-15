package game

import (
	"fmt"
	"testing"
	"time"

	"github.com/kyiku/hackz-ptera-back/internal/model"
	"github.com/kyiku/hackz-ptera-back/internal/queue"
	"github.com/kyiku/hackz-ptera-back/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCaptchaTimeout_Start(t *testing.T) {
	tests := []struct {
		name        string
		userStatus  string
		timeout     time.Duration
		wantRunning bool
	}{
		{
			name:        "正常系: CAPTCHAステージでタイマー開始",
			userStatus:  "stage2_captcha",
			timeout:     100 * time.Millisecond,
			wantRunning: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockConn := testutil.NewMockWebSocketConn()
			user := &model.User{
				ID:     "user1",
				Status: tt.userStatus,
				Conn:   mockConn,
			}

			timeout := NewCaptchaTimeout(user, tt.timeout)
			timeout.Start()
			defer timeout.Cancel()

			assert.Equal(t, tt.wantRunning, timeout.IsRunning())
		})
	}
}

func TestCaptchaTimeout_Cancel(t *testing.T) {
	tests := []struct {
		name               string
		cancelBeforeExpire bool
		wantConnClosed     bool
	}{
		{
			name:               "正常系: タイムアウト前にキャンセル（CAPTCHA成功時）",
			cancelBeforeExpire: true,
			wantConnClosed:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockConn := testutil.NewMockWebSocketConn()
			user := &model.User{
				ID:     "user1",
				Status: "stage2_captcha",
				Conn:   mockConn,
			}

			timeout := NewCaptchaTimeout(user, 200*time.Millisecond)
			timeout.Start()

			if tt.cancelBeforeExpire {
				// タイムアウト前にキャンセル
				timeout.Cancel()
			}

			// WaitForで接続状態を確認（閉じていないことを確認）
			err := testutil.WaitFor(100*time.Millisecond, 10*time.Millisecond, func() bool {
				return mockConn.IsClosed == tt.wantConnClosed
			})
			require.NoError(t, err, "接続状態が期待通りにならなかった")
		})
	}
}

func TestCaptchaTimeout_Expire(t *testing.T) {
	tests := []struct {
		name            string
		timeout         time.Duration
		wantMessageType string
		wantConnClosed  bool
		wantQueueReset  bool
		wantUserStatus  string
	}{
		{
			name:            "正常系: タイムアウト時に失敗通知・切断・最後尾へ",
			timeout:         50 * time.Millisecond,
			wantMessageType: "failure",
			wantConnClosed:  true,
			wantQueueReset:  true,
			wantUserStatus:  "waiting",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockConn := testutil.NewMockWebSocketConn()
			q := queue.NewWaitingQueue()

			user := &model.User{
				ID:     "user1",
				Status: "stage2_captcha",
				Conn:   mockConn,
			}

			timeout := NewCaptchaTimeout(user, tt.timeout)
			timeout.SetQueue(q)
			timeout.Start()

			// WaitForでタイムアウト処理完了を待機
			err := testutil.WaitFor(200*time.Millisecond, 10*time.Millisecond, func() bool {
				return mockConn.IsClosed
			})
			require.NoError(t, err, "タイムアウト処理が完了しなかった")

			// failureメッセージを確認
			msg := testutil.WaitForMessage(mockConn, 100*time.Millisecond)
			require.NotNil(t, msg, "メッセージが受信されなかった")

			assert.Equal(t, tt.wantMessageType, msg["type"])
			assert.Contains(t, msg["message"], "タイムアウト")
			assert.Equal(t, float64(3), msg["redirect_delay"])

			// 接続が閉じられたことを確認
			assert.Equal(t, tt.wantConnClosed, mockConn.IsClosed)

			// 待機列に追加されたことを確認
			if tt.wantQueueReset {
				assert.Equal(t, 1, q.Len())
			}

			// ユーザーステータスが更新されたことを確認
			assert.Equal(t, tt.wantUserStatus, user.Status)
		})
	}
}

func TestCaptchaTimeout_MultipleUsers(t *testing.T) {
	q := queue.NewWaitingQueue()
	users := make([]*model.User, 3)
	mockConns := make([]*testutil.MockWebSocketConn, 3)
	timeouts := make([]*CaptchaTimeout, 3)

	// 複数ユーザーのタイムアウトを設定
	for i := 0; i < 3; i++ {
		mockConns[i] = testutil.NewMockWebSocketConn()
		users[i] = &model.User{
			ID:     fmt.Sprintf("user%d", i),
			Status: "stage2_captcha",
			Conn:   mockConns[i],
		}
		timeouts[i] = NewCaptchaTimeout(users[i], 50*time.Millisecond)
		timeouts[i].SetQueue(q)
	}

	// 全員タイムアウト開始
	for _, timeout := range timeouts {
		timeout.Start()
	}

	// 全員タイムアウトするまで待機
	err := testutil.WaitFor(200*time.Millisecond, 10*time.Millisecond, func() bool {
		allClosed := true
		for _, conn := range mockConns {
			if !conn.IsClosed {
				allClosed = false
			}
		}
		return allClosed
	})
	require.NoError(t, err, "全ユーザーのタイムアウト処理が完了しなかった")

	// 全員待機列に追加されたことを確認
	assert.Equal(t, 3, q.Len())
}
