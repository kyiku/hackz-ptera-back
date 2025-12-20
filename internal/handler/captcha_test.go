package handler

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/kyiku/hackz-ptera-back/internal/model"
	"github.com/kyiku/hackz-ptera-back/internal/session"
	"github.com/kyiku/hackz-ptera-back/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCaptchaHandler_Generate(t *testing.T) {
	tests := []struct {
		name           string
		setupUser      func(*model.User)
		hasCookie      bool
		wantStatusCode int
		wantError      bool
		wantImageURL   bool
	}{
		{
			name: "正常系: CAPTCHA生成成功",
			setupUser: func(u *model.User) {
				u.Status = "registering"
			},
			hasCookie:      true,
			wantStatusCode: http.StatusOK,
			wantError:      false,
			wantImageURL:   true,
		},
		{
			name:           "異常系: セッションなし",
			setupUser:      nil,
			hasCookie:      false,
			wantStatusCode: http.StatusUnauthorized,
			wantError:      true,
			wantImageURL:   false,
		},
		{
			name: "異常系: waiting状態",
			setupUser: func(u *model.User) {
				u.Status = "waiting"
			},
			hasCookie:      true,
			wantStatusCode: http.StatusForbidden,
			wantError:      true,
			wantImageURL:   false,
		},
		{
			name: "異常系: stage1_dino状態",
			setupUser: func(u *model.User) {
				u.Status = "stage1_dino"
			},
			hasCookie:      true,
			wantStatusCode: http.StatusForbidden,
			wantError:      true,
			wantImageURL:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := session.NewSessionStore()
			mockS3 := testutil.NewMockS3Client()
			mockS3.Objects = map[string][]byte{
				"backgrounds/bg1.png":  testutil.CreateTestPNG(2816, 1536),
				"character/char1.png":  testutil.CreateTestPNG(100, 100),
				"character/char2.png":  testutil.CreateTestPNG(100, 100),
				"character/char3.png":  testutil.CreateTestPNG(100, 100),
				"character/char4.png":  testutil.CreateTestPNG(100, 100),
			}

			var sessionID string
			var user *model.User
			if tt.setupUser != nil {
				user, sessionID = store.Create()
				tt.setupUser(user)
			}

			h := NewCaptchaHandler(store, mockS3)

			tc := testutil.NewTestContext(http.MethodPost, "/api/captcha/generate", nil)
			if tt.hasCookie && sessionID != "" {
				tc.Request.AddCookie(&http.Cookie{Name: "session_id", Value: sessionID})
			}

			err := h.Generate(tc.Context)

			require.NoError(t, err)
			assert.Equal(t, tt.wantStatusCode, tc.Recorder.Code)

			var resp map[string]interface{}
			_ = json.Unmarshal(tc.Recorder.Body.Bytes(), &resp)

			assert.Equal(t, tt.wantError, resp["error"])

			if tt.wantImageURL {
				imageURL, ok := resp["image_url"].(string)
				assert.True(t, ok, "image_urlが存在するべき")
				assert.Contains(t, imageURL, "cloudfront.net/captcha/")

				// ターゲット座標が保存されていることを確認
				assert.NotZero(t, user.CaptchaTargetX)
				assert.NotZero(t, user.CaptchaTargetY)
			}
		})
	}
}

func TestCaptchaHandler_Generate_TargetPosition(t *testing.T) {
	store := session.NewSessionStore()
	mockS3 := testutil.NewMockS3Client()
	mockS3.Objects = map[string][]byte{
		"backgrounds/bg1.png":  testutil.CreateTestPNG(2816, 1536),
		"character/char1.png":  testutil.CreateTestPNG(100, 100),
		"character/char2.png":  testutil.CreateTestPNG(100, 100),
		"character/char3.png":  testutil.CreateTestPNG(100, 100),
		"character/char4.png":  testutil.CreateTestPNG(100, 100),
	}

	user, sessionID := store.Create()
	user.Status = "registering"

	h := NewCaptchaHandler(store, mockS3)

	tc := testutil.NewTestContext(http.MethodPost, "/api/captcha/generate", nil)
	tc.Request.AddCookie(&http.Cookie{Name: "session_id", Value: sessionID})

	err := h.Generate(tc.Context)
	require.NoError(t, err)

	// ターゲット座標が画像範囲内にあることを確認（中心座標）
	assert.GreaterOrEqual(t, user.CaptchaTargetX, 0)
	assert.Less(t, user.CaptchaTargetX, 2816)
	assert.GreaterOrEqual(t, user.CaptchaTargetY, 0)
	assert.Less(t, user.CaptchaTargetY, 1536)
}
