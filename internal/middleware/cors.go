// Package middleware provides HTTP middleware functions.
package middleware

import (
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
)

// CORSMiddleware returns a CORS middleware that allows requests from
// localhost and CloudFront domains.
func CORSMiddleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			origin := c.Request().Header.Get("Origin")

			// Check if origin is allowed
			if isAllowedOrigin(origin) {
				c.Response().Header().Set("Access-Control-Allow-Origin", origin)
				c.Response().Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
				c.Response().Header().Set("Access-Control-Allow-Headers", "Content-Type")
				c.Response().Header().Set("Access-Control-Allow-Credentials", "true")
			}

			// Handle preflight requests
			if c.Request().Method == http.MethodOptions {
				return c.NoContent(http.StatusNoContent)
			}

			return next(c)
		}
	}
}

// isAllowedOrigin checks if the origin is allowed for CORS.
func isAllowedOrigin(origin string) bool {
	if origin == "" {
		return false
	}

	// Allow localhost for development
	if strings.HasPrefix(origin, "http://localhost:") {
		return true
	}

	// Allow CloudFront domains
	if strings.HasSuffix(origin, ".cloudfront.net") {
		return true
	}

	return false
}
