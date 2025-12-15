package model

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestUser_NewUser(t *testing.T) {
	tests := []struct {
		name           string
		wantStatus     string
		wantNonEmptyID bool
	}{
		{
			name:           "正常系: 新規ユーザー作成",
			wantStatus:     "waiting",
			wantNonEmptyID: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user := NewUser()

			assert.NotEmpty(t, user.ID)
			assert.Equal(t, tt.wantStatus, user.Status)
			assert.False(t, user.JoinedAt.IsZero())

			// UUIDフォーマットの確認
			_, err := uuid.Parse(user.ID)
			assert.NoError(t, err, "IDはUUID形式であるべき")
		})
	}
}

func TestUser_StatusTransitions(t *testing.T) {
	tests := []struct {
		name       string
		fromStatus string
		toStatus   string
		wantValid  bool
	}{
		// 正常な遷移
		{name: "waiting -> stage1_dino", fromStatus: "waiting", toStatus: "stage1_dino", wantValid: true},
		{name: "stage1_dino -> stage2_captcha", fromStatus: "stage1_dino", toStatus: "stage2_captcha", wantValid: true},
		{name: "stage2_captcha -> registering", fromStatus: "stage2_captcha", toStatus: "registering", wantValid: true},
		{name: "registering -> waiting (失敗時)", fromStatus: "registering", toStatus: "waiting", wantValid: true},
		{name: "stage1_dino -> waiting (失敗時)", fromStatus: "stage1_dino", toStatus: "waiting", wantValid: true},
		{name: "stage2_captcha -> waiting (失敗時)", fromStatus: "stage2_captcha", toStatus: "waiting", wantValid: true},

		// 不正な遷移
		{name: "waiting -> registering (不正)", fromStatus: "waiting", toStatus: "registering", wantValid: false},
		{name: "stage1_dino -> registering (不正)", fromStatus: "stage1_dino", toStatus: "registering", wantValid: false},
		{name: "waiting -> stage2_captcha (不正)", fromStatus: "waiting", toStatus: "stage2_captcha", wantValid: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user := NewUser()
			user.Status = tt.fromStatus

			valid := user.CanTransitionTo(tt.toStatus)
			assert.Equal(t, tt.wantValid, valid)
		})
	}
}

func TestUser_ResetToWaiting(t *testing.T) {
	tests := []struct {
		name             string
		initialStatus    string
		captchaAttempts  int
		otpAttempts      int
		otpFishName      string
		registerToken    string
		wantStatus       string
		wantCaptchaReset bool
		wantOTPReset     bool
		wantTokenReset   bool
	}{
		{
			name:             "正常系: registering状態からリセット",
			initialStatus:    "registering",
			captchaAttempts:  2,
			otpAttempts:      1,
			otpFishName:      "オニカマス",
			registerToken:    "token-123",
			wantStatus:       "waiting",
			wantCaptchaReset: true,
			wantOTPReset:     true,
			wantTokenReset:   true,
		},
		{
			name:             "正常系: stage1_dino状態からリセット",
			initialStatus:    "stage1_dino",
			captchaAttempts:  0,
			otpAttempts:      0,
			otpFishName:      "",
			registerToken:    "",
			wantStatus:       "waiting",
			wantCaptchaReset: true,
			wantOTPReset:     true,
			wantTokenReset:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user := NewUser()
			user.Status = tt.initialStatus
			user.CaptchaAttempts = tt.captchaAttempts
			user.OTPAttempts = tt.otpAttempts
			user.OTPFishName = tt.otpFishName
			user.RegisterToken = tt.registerToken
			user.RegisterTokenExp = time.Now().Add(10 * time.Minute)

			user.ResetToWaiting()

			assert.Equal(t, tt.wantStatus, user.Status)

			if tt.wantCaptchaReset {
				assert.Zero(t, user.CaptchaAttempts)
				assert.Zero(t, user.CaptchaTargetX)
				assert.Zero(t, user.CaptchaTargetY)
			}

			if tt.wantOTPReset {
				assert.Zero(t, user.OTPAttempts)
				assert.Empty(t, user.OTPFishName)
			}

			if tt.wantTokenReset {
				assert.Empty(t, user.RegisterToken)
				assert.True(t, user.RegisterTokenExp.IsZero())
			}
		})
	}
}

func TestUser_SetCaptchaTarget(t *testing.T) {
	tests := []struct {
		name    string
		targetX int
		targetY int
	}{
		{name: "正常系: 座標設定", targetX: 512, targetY: 384},
		{name: "境界値: 左上", targetX: 0, targetY: 0},
		{name: "境界値: 右下", targetX: 1023, targetY: 767},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user := NewUser()

			user.SetCaptchaTarget(tt.targetX, tt.targetY)

			assert.Equal(t, tt.targetX, user.CaptchaTargetX)
			assert.Equal(t, tt.targetY, user.CaptchaTargetY)
		})
	}
}

func TestUser_IncrementAttempts(t *testing.T) {
	tests := []struct {
		name            string
		attemptType     string
		initialAttempts int
		wantAttempts    int
		wantExceeded    bool
	}{
		{name: "CAPTCHA: 1回目", attemptType: "captcha", initialAttempts: 0, wantAttempts: 1, wantExceeded: false},
		{name: "CAPTCHA: 2回目", attemptType: "captcha", initialAttempts: 1, wantAttempts: 2, wantExceeded: false},
		{name: "CAPTCHA: 3回目（上限）", attemptType: "captcha", initialAttempts: 2, wantAttempts: 3, wantExceeded: true},
		{name: "OTP: 1回目", attemptType: "otp", initialAttempts: 0, wantAttempts: 1, wantExceeded: false},
		{name: "OTP: 3回目（上限）", attemptType: "otp", initialAttempts: 2, wantAttempts: 3, wantExceeded: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user := NewUser()

			if tt.attemptType == "captcha" {
				user.CaptchaAttempts = tt.initialAttempts
				exceeded := user.IncrementCaptchaAttempts()
				assert.Equal(t, tt.wantAttempts, user.CaptchaAttempts)
				assert.Equal(t, tt.wantExceeded, exceeded)
			} else {
				user.OTPAttempts = tt.initialAttempts
				exceeded := user.IncrementOTPAttempts()
				assert.Equal(t, tt.wantAttempts, user.OTPAttempts)
				assert.Equal(t, tt.wantExceeded, exceeded)
			}
		})
	}
}
