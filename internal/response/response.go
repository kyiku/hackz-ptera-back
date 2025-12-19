// Package response provides helpers for consistent API responses.
package response

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

// Success sends a successful JSON response with the given data.
// The response will always include "error": false.
func Success(c echo.Context, data map[string]interface{}) error {
	resp := make(map[string]interface{})
	resp["error"] = false

	// Merge additional data
	for k, v := range data {
		resp[k] = v
	}

	return c.JSON(http.StatusOK, resp)
}

// Error sends an error JSON response with the given status code and message.
func Error(c echo.Context, statusCode int, message string) error {
	return c.JSON(statusCode, map[string]interface{}{
		"error":   true,
		"message": message,
	})
}

// ErrorWithRedirect sends an error response with a redirect delay.
// This is used when the client should redirect after a failure.
func ErrorWithRedirect(c echo.Context, message string, redirectDelay int) error {
	return c.JSON(http.StatusOK, map[string]interface{}{
		"error":          true,
		"message":        message,
		"redirect_delay": redirectDelay,
	})
}

// ErrorWithCode sends an error response with a specific error code.
// This is useful for clients that need to handle specific error types.
func ErrorWithCode(c echo.Context, statusCode int, code string, message string) error {
	return c.JSON(statusCode, map[string]interface{}{
		"error":   true,
		"code":    code,
		"message": message,
	})
}
