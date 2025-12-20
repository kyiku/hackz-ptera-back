package main

import (
	"log"
	"net/http"
	"os"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/kyiku/hackz-ptera-back/internal/handler"
	"github.com/kyiku/hackz-ptera-back/internal/queue"
	"github.com/kyiku/hackz-ptera-back/internal/session"
)

func main() {
	e := echo.New()

	// Middleware
	e.Use(middleware.RequestLoggerWithConfig(middleware.RequestLoggerConfig{
		LogStatus: true,
		LogURI:    true,
		LogMethod: true,
		LogValuesFunc: func(c echo.Context, v middleware.RequestLoggerValues) error {
			e.Logger.Infof("%s %s %d", v.Method, v.URI, v.Status)
			return nil
		},
	}))
	e.Use(middleware.Recover())
	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{http.MethodGet, http.MethodPost, http.MethodOptions},
		AllowHeaders:     []string{"*"},
		AllowCredentials: true,
	}))

	// Initialize dependencies
	sessionStore := session.NewSessionStore()
	waitingQueue := queue.NewWaitingQueue()

	// Initialize handlers
	wsHandler := handler.NewWebSocketHandler(sessionStore, waitingQueue)

	// Health check (root level for ALB)
	e.GET("/health", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{
			"status": "ok",
		})
	})

	// WebSocket endpoint
	e.GET("/ws", wsHandler.Connect)

	// API routes
	api := e.Group("/api")
	api.GET("/health", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{
			"status": "ok",
		})
	})

	// Queue status endpoint (for debugging)
	api.GET("/queue/status", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"queue_length": waitingQueue.Len(),
		})
	})

	// Get port from environment or default
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// Start server
	log.Printf("Starting server on :%s", port)
	e.Logger.Fatal(e.Start(":" + port))
}
