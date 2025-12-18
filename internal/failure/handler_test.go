package failure

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

func TestFailureHandler_HandleFailure(t *testing.T) {
	tests := []struct {
		name           string
		userStatus     string
		message        string
		wantMsgType    string
		wantRedirect   float64
		wantConnClosed bool
		wantQueueLen   int
		wantNewStatus  string
	}{
		{
			name:           "Dino Run失敗",
			userStatus:     "stage1_dino",
			message:        "ゲームオーバー。待機列の最後尾からやり直しです。",
			wantMsgType:    "failure",
			wantRedirect:   3,
			wantConnClosed: true,
			wantQueueLen:   1,
			wantNewStatus:  "waiting",
		},
		{
			name:           "CAPTCHA失敗",
			userStatus:     "registering",
			message:        "3回失敗しました。待機列の最後尾からやり直しです。",
			wantMsgType:    "failure",
			wantRedirect:   3,
			wantConnClosed: true,
			wantQueueLen:   1,
			wantNewStatus:  "waiting",
		},
		{
			name:           "OTP失敗",
			userStatus:     "registering",
			message:        "魚の名前を3回間違えました。",
			wantMsgType:    "failure",
			wantRedirect:   3,
			wantConnClosed: true,
			wantQueueLen:   1,
			wantNewStatus:  "waiting",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			q := queue.NewWaitingQueue()
			mockConn := testutil.NewMockWebSocketConn()

			user := &model.User{
				ID:     "user1",
				Status: tt.userStatus,
				Conn:   mockConn,
			}

			handler := NewFailureHandler(q)

			err := handler.HandleFailure(user, tt.message)
			require.NoError(t, err)

			// WaitForで接続が閉じられることを確認
			err = testutil.WaitFor(100*time.Millisecond, 10*time.Millisecond, func() bool {
				return mockConn.IsClosed
			})
			require.NoError(t, err, "WebSocket接続が閉じられるべき")

			// 失敗メッセージが送信されたことを確認
			msg := testutil.WaitForMessage(mockConn, 100*time.Millisecond)
			require.NotNil(t, msg, "メッセージが受信されるべき")

			assert.Equal(t, tt.wantMsgType, msg["type"])
			assert.Equal(t, tt.wantRedirect, msg["redirectDelay"])

			// 待機列に追加、ステータスリセット
			assert.Equal(t, tt.wantQueueLen, q.Len())
			assert.Equal(t, tt.wantNewStatus, user.Status)
		})
	}
}

func TestFailureHandler_ResetUserState(t *testing.T) {
	tests := []struct {
		name            string
		initialStatus   string
		captchaAttempts int
		otpAttempts     int
		otpFishName     string
		registerToken   string
		wantAllReset    bool
	}{
		{
			name:            "registering状態から完全リセット",
			initialStatus:   "registering",
			captchaAttempts: 2,
			otpAttempts:     1,
			otpFishName:     "オニカマス",
			registerToken:   "token",
			wantAllReset:    true,
		},
		{
			name:            "registering状態からリセット（CAPTCHA失敗）",
			initialStatus:   "registering",
			captchaAttempts: 3,
			otpAttempts:     0,
			otpFishName:     "",
			registerToken:   "",
			wantAllReset:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			q := queue.NewWaitingQueue()
			mockConn := testutil.NewMockWebSocketConn()

			user := &model.User{
				ID:              "user1",
				Status:          tt.initialStatus,
				CaptchaAttempts: tt.captchaAttempts,
				OTPAttempts:     tt.otpAttempts,
				OTPFishName:     tt.otpFishName,
				RegisterToken:   tt.registerToken,
				Conn:            mockConn,
			}

			handler := NewFailureHandler(q)
			handler.HandleFailure(user, "失敗")

			// WaitForで処理完了を待機
			err := testutil.WaitFor(100*time.Millisecond, 10*time.Millisecond, func() bool {
				return mockConn.IsClosed
			})
			require.NoError(t, err)

			// すべての状態がリセットされていることを確認
			assert.Equal(t, "waiting", user.Status)
			assert.Zero(t, user.CaptchaAttempts)
			assert.Zero(t, user.OTPAttempts)
			assert.Empty(t, user.OTPFishName)
			assert.Empty(t, user.RegisterToken)
		})
	}
}

func TestFailureHandler_MultipleUsers(t *testing.T) {
	q := queue.NewWaitingQueue()
	users := make([]*model.User, 3)
	mockConns := make([]*testutil.MockWebSocketConn, 3)

	// 複数ユーザーを設定
	for i := 0; i < 3; i++ {
		mockConns[i] = testutil.NewMockWebSocketConn()
		users[i] = &model.User{
			ID:     fmt.Sprintf("user%d", i),
			Status: "stage1_dino",
			Conn:   mockConns[i],
		}
	}

	handler := NewFailureHandler(q)

	// 全員失敗処理
	for _, user := range users {
		err := handler.HandleFailure(user, "ゲームオーバー")
		require.NoError(t, err)
	}

	// WaitForで全員の接続が閉じられることを確認
	err := testutil.WaitFor(200*time.Millisecond, 10*time.Millisecond, func() bool {
		allClosed := true
		for _, conn := range mockConns {
			if !conn.IsClosed {
				allClosed = false
			}
		}
		return allClosed
	})
	require.NoError(t, err, "全ユーザーの接続が閉じられるべき")

	// 全員待機列に追加されたことを確認
	assert.Equal(t, 3, q.Len())

	// 全員のステータスがwaitingになっていることを確認
	for _, user := range users {
		assert.Equal(t, "waiting", user.Status)
	}
}
