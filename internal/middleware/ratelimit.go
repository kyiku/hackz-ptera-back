// Package middleware provides HTTP middleware functions.
package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/labstack/echo/v4"
)

// RateLimiter tracks request counts per IP.
type RateLimiter struct {
	requests map[string]*requestInfo
	mu       sync.RWMutex
	limit    int           // max requests per window
	window   time.Duration // time window
}

type requestInfo struct {
	count     int
	resetTime time.Time
}

// NewRateLimiter creates a new RateLimiter.
func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	rl := &RateLimiter{
		requests: make(map[string]*requestInfo),
		limit:    limit,
		window:   window,
	}

	// Start cleanup goroutine
	go rl.cleanup()

	return rl
}

// cleanup periodically removes expired entries.
func (rl *RateLimiter) cleanup() {
	ticker := time.NewTicker(rl.window)
	defer ticker.Stop()

	for range ticker.C {
		rl.mu.Lock()
		now := time.Now()
		for ip, info := range rl.requests {
			if now.After(info.resetTime) {
				delete(rl.requests, ip)
			}
		}
		rl.mu.Unlock()
	}
}

// isAllowed checks if a request from the given IP is allowed.
func (rl *RateLimiter) isAllowed(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	info, exists := rl.requests[ip]

	if !exists || now.After(info.resetTime) {
		// New window
		rl.requests[ip] = &requestInfo{
			count:     1,
			resetTime: now.Add(rl.window),
		}
		return true
	}

	if info.count >= rl.limit {
		return false
	}

	info.count++
	return true
}

// RateLimitMiddleware returns a rate limiting middleware.
func RateLimitMiddleware(limit int, window time.Duration) echo.MiddlewareFunc {
	limiter := NewRateLimiter(limit, window)

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			ip := c.RealIP()

			if !limiter.isAllowed(ip) {
				return c.JSON(http.StatusTooManyRequests, map[string]interface{}{
					"error":   true,
					"message": "リクエストが多すぎます。しばらく待ってから再試行してください。",
				})
			}

			return next(c)
		}
	}
}
