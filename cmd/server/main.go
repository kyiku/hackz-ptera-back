package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/kyiku/hackz-ptera-back/internal/handler"
	"github.com/kyiku/hackz-ptera-back/internal/model"
	"github.com/kyiku/hackz-ptera-back/internal/queue"
	"github.com/kyiku/hackz-ptera-back/internal/session"
)

// S3Adapter adapts AWS S3 client to our interface
type S3Adapter struct {
	client *s3.Client
	bucket string
}

func (a *S3Adapter) GetObject(key string) ([]byte, error) {
	output, err := a.client.GetObject(context.TODO(), &s3.GetObjectInput{
		Bucket: &a.bucket,
		Key:    &key,
	})
	if err != nil {
		return nil, err
	}
	defer output.Body.Close()
	return io.ReadAll(output.Body)
}

func (a *S3Adapter) PutObject(key string, data []byte) error {
	_, err := a.client.PutObject(context.TODO(), &s3.PutObjectInput{
		Bucket: &a.bucket,
		Key:    &key,
		Body:   bytes.NewReader(data),
	})
	return err
}

func (a *S3Adapter) ListObjects(prefix string) ([]string, error) {
	output, err := a.client.ListObjectsV2(context.TODO(), &s3.ListObjectsV2Input{
		Bucket: &a.bucket,
		Prefix: &prefix,
	})
	if err != nil {
		return nil, err
	}

	keys := make([]string, 0, len(output.Contents))
	for _, obj := range output.Contents {
		keys = append(keys, *obj.Key)
	}
	return keys, nil
}

// BedrockAdapter adapts AWS Bedrock client to our interface
type BedrockAdapter struct {
	client *bedrockruntime.Client
}

// BedrockRequest represents the request body for Claude via Bedrock
type BedrockRequest struct {
	AnthropicVersion string           `json:"anthropic_version"`
	MaxTokens        int              `json:"max_tokens"`
	Messages         []BedrockMessage `json:"messages"`
}

// BedrockMessage represents a message in the Bedrock request
type BedrockMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

func (a *BedrockAdapter) InvokeModel(modelID string, prompt string) (string, error) {
	// Build request body for Claude using proper JSON marshaling
	req := BedrockRequest{
		AnthropicVersion: "bedrock-2023-05-31",
		MaxTokens:        256,
		Messages: []BedrockMessage{
			{Role: "user", Content: prompt},
		},
	}

	body, err := json.Marshal(req)
	if err != nil {
		return "", err
	}

	output, err := a.client.InvokeModel(context.TODO(), &bedrockruntime.InvokeModelInput{
		ModelId:     &modelID,
		Body:        body,
		ContentType: stringPtr("application/json"),
	})
	if err != nil {
		return "", err
	}

	return string(output.Body), nil
}

func stringPtr(s string) *string {
	return &s
}

// QueueAdapter adapts WaitingQueue to QueueInterfaceForCaptcha
type QueueAdapter struct {
	queue *queue.WaitingQueue
}

func (a *QueueAdapter) Add(userID string, conn model.WebSocketConn) {
	a.queue.Add(userID, conn)
}

func (a *QueueAdapter) Remove(userID string) {
	a.queue.Remove(userID)
}

func (a *QueueAdapter) BroadcastPositions() {
	a.queue.BroadcastPositions()
}

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
	// CORS configuration - AllowOrigins cannot be "*" when AllowCredentials is true
	corsOrigin := os.Getenv("CORS_ORIGIN")
	if corsOrigin == "" {
		corsOrigin = "http://localhost:5173"
	}
	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins:     []string{corsOrigin, "https://d3qfj76e9d3p81.cloudfront.net"},
		AllowMethods:     []string{http.MethodGet, http.MethodPost, http.MethodOptions},
		AllowHeaders:     []string{"Content-Type", "Authorization"},
		AllowCredentials: true,
	}))

	// Initialize dependencies
	sessionStore := session.NewSessionStore()
	waitingQueue := queue.NewWaitingQueue()

	// Load AWS config
	region := os.Getenv("AWS_REGION")
	if region == "" {
		region = "ap-northeast-1"
	}

	cfg, err := config.LoadDefaultConfig(context.TODO(), config.WithRegion(region))
	if err != nil {
		log.Printf("Warning: Failed to load AWS config: %v (some features may not work)", err)
	}

	// S3 client
	bucket := os.Getenv("S3_BUCKET")
	if bucket == "" {
		bucket = "hackz-ptera-assets"
	}

	cloudfrontURL := os.Getenv("CLOUDFRONT_URL")
	if cloudfrontURL == "" {
		cloudfrontURL = "https://test.cloudfront.net"
	}

	var s3Adapter *S3Adapter
	if err == nil {
		s3Client := s3.NewFromConfig(cfg)
		s3Adapter = &S3Adapter{
			client: s3Client,
			bucket: bucket,
		}
	}

	// Bedrock client
	var bedrockAdapter *BedrockAdapter
	if err == nil {
		bedrockClient := bedrockruntime.NewFromConfig(cfg)
		bedrockAdapter = &BedrockAdapter{
			client: bedrockClient,
		}
	}

	// Queue adapter
	queueAdapter := &QueueAdapter{queue: waitingQueue}

	// Initialize handlers
	wsHandler := handler.NewWebSocketHandler(sessionStore, waitingQueue)
	dinoHandler := handler.NewDinoHandler(sessionStore)
	dinoHandler.SetQueue(queueAdapter)
	registerHandler := handler.NewRegisterHandler(sessionStore)
	registerHandler.SetQueue(queueAdapter)

	// Handlers that require S3
	var captchaHandler *handler.CaptchaHandler
	var otpHandler *handler.OTPHandler
	if s3Adapter != nil {
		captchaHandler = handler.NewCaptchaHandler(sessionStore, s3Adapter)
		captchaHandler.SetCloudfrontURL(cloudfrontURL)
		captchaHandler.SetQueue(queueAdapter)

		otpHandler = handler.NewOTPHandler(sessionStore, s3Adapter)
		otpHandler.SetQueue(queueAdapter)
	}

	// Handlers that require Bedrock
	var passwordHandler *handler.PasswordHandler
	if bedrockAdapter != nil {
		passwordHandler = handler.NewPasswordHandler(sessionStore, bedrockAdapter)
		passwordHandler.EnableFallback(true) // Use fallback if Bedrock fails
	}

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

	// Health check
	api.GET("/health", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{
			"status": "ok",
		})
	})

	// Queue status (debug)
	api.GET("/queue/status", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"queue_length": waitingQueue.Len(),
		})
	})

	// Game endpoints
	api.POST("/game/dino/start", dinoHandler.Start)
	api.POST("/game/dino/result", dinoHandler.Result)

	// CAPTCHA endpoints
	if captchaHandler != nil {
		api.POST("/captcha/generate", captchaHandler.Generate)
		api.POST("/captcha/verify", captchaHandler.Verify)
	} else {
		api.POST("/captcha/generate", unavailableHandler("S3"))
		api.POST("/captcha/verify", unavailableHandler("S3"))
	}

	// OTP endpoints
	if otpHandler != nil {
		api.POST("/otp/send", otpHandler.Send)
		api.POST("/otp/verify", otpHandler.Verify)
	} else {
		api.POST("/otp/send", unavailableHandler("S3"))
		api.POST("/otp/verify", unavailableHandler("S3"))
	}

	// Password analysis endpoint
	if passwordHandler != nil {
		api.POST("/password/analyze", passwordHandler.Analyze)
	} else {
		api.POST("/password/analyze", unavailableHandler("Bedrock"))
	}

	// Registration endpoint
	api.POST("/register", registerHandler.Submit)

	// Get port from environment or default
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// Log registered endpoints
	log.Println("Registered endpoints:")
	log.Println("  GET  /health")
	log.Println("  GET  /ws")
	log.Println("  GET  /api/health")
	log.Println("  GET  /api/queue/status")
	log.Println("  POST /api/game/dino/start")
	log.Println("  POST /api/game/dino/result")
	log.Println("  POST /api/captcha/generate")
	log.Println("  POST /api/captcha/verify")
	log.Println("  POST /api/otp/send")
	log.Println("  POST /api/otp/verify")
	log.Println("  POST /api/password/analyze")
	log.Println("  POST /api/register")

	// Start server
	log.Printf("Starting server on :%s", port)
	e.Logger.Fatal(e.Start(":" + port))
}

// unavailableHandler returns a handler that responds with service unavailable
func unavailableHandler(service string) echo.HandlerFunc {
	return func(c echo.Context) error {
		return c.JSON(http.StatusServiceUnavailable, map[string]interface{}{
			"error":   true,
			"message": service + " is not configured",
		})
	}
}
