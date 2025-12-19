package middleware

import (
	"net/http"
	"testing"

	"hackz-ptera/back/internal/testutil"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
)

func TestCORSMiddleware(t *testing.T) {
	tests := []struct {
		name               string
		origin             string
		method             string
		wantAllowOrigin    string
		wantAllowMethods   string
		wantAllowHeaders   string
		wantAllowCreds     string
		wantOptionsStatus  int
	}{
		{
			name:              "正常系: 許可されたオリジン",
			origin:            "https://example.cloudfront.net",
			method:            http.MethodGet,
			wantAllowOrigin:   "https://example.cloudfront.net",
			wantAllowMethods:  "GET, POST, OPTIONS",
			wantAllowHeaders:  "Content-Type",
			wantAllowCreds:    "true",
			wantOptionsStatus: http.StatusNoContent,
		},
		{
			name:              "正常系: localhostオリジン",
			origin:            "http://localhost:3000",
			method:            http.MethodGet,
			wantAllowOrigin:   "http://localhost:3000",
			wantAllowMethods:  "GET, POST, OPTIONS",
			wantAllowHeaders:  "Content-Type",
			wantAllowCreds:    "true",
			wantOptionsStatus: http.StatusNoContent,
		},
		{
			name:              "正常系: PREFLIGHTリクエスト",
			origin:            "https://example.cloudfront.net",
			method:            http.MethodOptions,
			wantAllowOrigin:   "https://example.cloudfront.net",
			wantAllowMethods:  "GET, POST, OPTIONS",
			wantAllowHeaders:  "Content-Type",
			wantAllowCreds:    "true",
			wantOptionsStatus: http.StatusNoContent,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tc := testutil.NewTestContext(tt.method, "/api/test", nil)
			tc.Request.Header.Set("Origin", tt.origin)

			middleware := CORSMiddleware()

			handler := middleware(func(c echo.Context) error {
				return c.String(http.StatusOK, "ok")
			})

			err := handler(tc.Context)

			if tt.method == http.MethodOptions {
				assert.Equal(t, tt.wantOptionsStatus, tc.Recorder.Code)
			} else {
				assert.NoError(t, err)
			}

			assert.Equal(t, tt.wantAllowOrigin, tc.Recorder.Header().Get("Access-Control-Allow-Origin"))
			assert.Contains(t, tc.Recorder.Header().Get("Access-Control-Allow-Methods"), "GET")
			assert.Contains(t, tc.Recorder.Header().Get("Access-Control-Allow-Methods"), "POST")
			assert.Equal(t, tt.wantAllowCreds, tc.Recorder.Header().Get("Access-Control-Allow-Credentials"))
		})
	}
}

func TestCORSMiddleware_InvalidOrigin(t *testing.T) {
	tc := testutil.NewTestContext(http.MethodGet, "/api/test", nil)
	tc.Request.Header.Set("Origin", "https://malicious-site.com")

	middleware := CORSMiddleware()

	handler := middleware(func(c echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})

	_ = handler(tc.Context)

	// 不正なオリジンにはCORSヘッダーが設定されない
	allowOrigin := tc.Recorder.Header().Get("Access-Control-Allow-Origin")
	assert.NotEqual(t, "https://malicious-site.com", allowOrigin)
}
