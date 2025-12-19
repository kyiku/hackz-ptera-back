package stage

import (
	"testing"
	"time"

	"hackz-ptera/back/internal/model"
	"hackz-ptera/back/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStageTransition_ValidTransitions(t *testing.T) {
	tests := []struct {
		name       string
		fromStatus string
		toStatus   string
		wantValid  bool
		wantCode   string
	}{
		{
			name:       "waiting -> stage1_dino",
			fromStatus: "waiting",
			toStatus:   "stage1_dino",
			wantValid:  true,
			wantCode:   "",
		},
		{
			name:       "stage1_dino -> stage2_captcha",
			fromStatus: "stage1_dino",
			toStatus:   "stage2_captcha",
			wantValid:  true,
			wantCode:   "",
		},
		{
			name:       "stage2_captcha -> registering",
			fromStatus: "stage2_captcha",
			toStatus:   "registering",
			wantValid:  true,
			wantCode:   "",
		},
		{
			name:       "registering -> waiting（失敗時）",
			fromStatus: "registering",
			toStatus:   "waiting",
			wantValid:  true,
			wantCode:   "",
		},
		{
			name:       "waiting -> registering（不正）",
			fromStatus: "waiting",
			toStatus:   "registering",
			wantValid:  false,
			wantCode:   "INVALID_TRANSITION",
		},
		{
			name:       "stage1_dino -> registering（不正）",
			fromStatus: "stage1_dino",
			toStatus:   "registering",
			wantValid:  false,
			wantCode:   "INVALID_TRANSITION",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user := &model.User{
				ID:     "user1",
				Status: tt.fromStatus,
			}

			manager := NewTransitionManager()
			valid, errCode := manager.CanTransition(user, tt.toStatus)

			assert.Equal(t, tt.wantValid, valid)
			if !tt.wantValid {
				assert.Equal(t, tt.wantCode, errCode)
			}
		})
	}
}

func TestStageTransition_Execute(t *testing.T) {
	tests := []struct {
		name           string
		fromStatus     string
		toStatus       string
		wantMsgType    string
		wantNextStage  string
	}{
		{
			name:          "waiting -> stage1_dino",
			fromStatus:    "waiting",
			toStatus:      "stage1_dino",
			wantMsgType:   "stage_change",
			wantNextStage: "stage1_dino",
		},
		{
			name:          "stage1_dino -> stage2_captcha",
			fromStatus:    "stage1_dino",
			toStatus:      "stage2_captcha",
			wantMsgType:   "stage_change",
			wantNextStage: "stage2_captcha",
		},
		{
			name:          "stage2_captcha -> registering",
			fromStatus:    "stage2_captcha",
			toStatus:      "registering",
			wantMsgType:   "stage_change",
			wantNextStage: "registering",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockConn := testutil.NewMockWebSocketConn()
			user := &model.User{
				ID:     "user1",
				Status: tt.fromStatus,
				Conn:   mockConn,
			}

			manager := NewTransitionManager()
			err := manager.Execute(user, tt.toStatus)

			require.NoError(t, err)
			assert.Equal(t, tt.toStatus, user.Status)

			// WebSocketメッセージを確認
			err = testutil.WaitFor(100*time.Millisecond, 10*time.Millisecond, func() bool {
				return mockConn.LastMessage != nil
			})
			require.NoError(t, err)

			msg := mockConn.GetLastMessageAsMap()
			require.NotNil(t, msg)

			assert.Equal(t, tt.wantMsgType, msg["type"])
			assert.Equal(t, tt.wantNextStage, msg["stage"])
		})
	}
}

func TestStageTransition_InvalidExecute(t *testing.T) {
	mockConn := testutil.NewMockWebSocketConn()
	user := &model.User{
		ID:     "user1",
		Status: "waiting",
		Conn:   mockConn,
	}

	manager := NewTransitionManager()
	err := manager.Execute(user, "registering") // 不正な遷移

	assert.Error(t, err)
	assert.Equal(t, "waiting", user.Status) // ステータスは変わらない
}

func TestStageTransition_WebSocketMessage(t *testing.T) {
	tests := []struct {
		name        string
		stage       string
		wantMessage string
	}{
		{
			name:        "Dino Runステージ",
			stage:       "stage1_dino",
			wantMessage: "Dino Run ゲームを開始してください",
		},
		{
			name:        "CAPTCHAステージ",
			stage:       "stage2_captcha",
			wantMessage: "CAPTCHAを解いてください",
		},
		{
			name:        "登録ステージ",
			stage:       "registering",
			wantMessage: "登録フォームに入力してください",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockConn := testutil.NewMockWebSocketConn()
			user := &model.User{
				ID:     "user1",
				Status: getPreviousStage(tt.stage),
				Conn:   mockConn,
			}

			manager := NewTransitionManager()
			manager.Execute(user, tt.stage)

			err := testutil.WaitFor(100*time.Millisecond, 10*time.Millisecond, func() bool {
				return mockConn.LastMessage != nil
			})
			require.NoError(t, err)

			msg := mockConn.GetLastMessageAsMap()
			assert.Contains(t, msg["message"], tt.wantMessage)
		})
	}
}

// getPreviousStage returns the valid previous stage for transition testing.
func getPreviousStage(stage string) string {
	switch stage {
	case "stage1_dino":
		return "waiting"
	case "stage2_captcha":
		return "stage1_dino"
	case "registering":
		return "stage2_captcha"
	default:
		return "waiting"
	}
}
