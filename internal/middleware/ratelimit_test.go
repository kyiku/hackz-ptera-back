package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
)

func TestRateLimitMiddleware(t *testing.T) {
	e := echo.New()

	// Create middleware with limit of 3 requests per second
	rateLimitMW := RateLimitMiddleware(3, 1*time.Second)

	handler := func(c echo.Context) error {
		return c.String(http.StatusOK, "OK")
	}

	t.Run("allows requests within limit", func(t *testing.T) {
		for i := 0; i < 3; i++ {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.RemoteAddr = "192.168.1.1:12345"
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)

			err := rateLimitMW(handler)(c)

			assert.NoError(t, err)
			assert.Equal(t, http.StatusOK, rec.Code)
		}
	})

	t.Run("blocks requests exceeding limit", func(t *testing.T) {
		// Use a different IP to avoid interference from previous test
		for i := 0; i < 3; i++ {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.RemoteAddr = "192.168.1.2:12345"
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)

			_ = rateLimitMW(handler)(c)
		}

		// Fourth request should be blocked
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "192.168.1.2:12345"
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		err := rateLimitMW(handler)(c)

		assert.NoError(t, err)
		assert.Equal(t, http.StatusTooManyRequests, rec.Code)
	})

	t.Run("allows requests from different IPs", func(t *testing.T) {
		ips := []string{"10.0.0.1:12345", "10.0.0.2:12345", "10.0.0.3:12345"}

		for _, ip := range ips {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.RemoteAddr = ip
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)

			err := rateLimitMW(handler)(c)

			assert.NoError(t, err)
			assert.Equal(t, http.StatusOK, rec.Code)
		}
	})
}

func TestRateLimiter_WindowReset(t *testing.T) {
	limiter := NewRateLimiter(2, 100*time.Millisecond)
	ip := "test-ip"

	// Use up the limit
	assert.True(t, limiter.isAllowed(ip))
	assert.True(t, limiter.isAllowed(ip))
	assert.False(t, limiter.isAllowed(ip))

	// Wait for window to reset
	time.Sleep(150 * time.Millisecond)

	// Should be allowed again
	assert.True(t, limiter.isAllowed(ip))
}
