// CAPTCHA test server
package main

import (
	"bytes"
	"context"
	"io"
	"log"
	"net/http"
	"os"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/kyiku/hackz-ptera-back/internal/handler"
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


func main() {
	// Load AWS config
	cfg, err := config.LoadDefaultConfig(context.TODO(), config.WithRegion("ap-northeast-1"))
	if err != nil {
		log.Fatal("Failed to load AWS config:", err)
	}

	// Create S3 client
	s3Client := s3.NewFromConfig(cfg)
	bucket := os.Getenv("S3_BUCKET")
	if bucket == "" {
		bucket = "hackz-ptera-assets"
	}
	// Use local proxy for testing (avoids S3 permission issues)
	cloudfrontURL := "http://localhost:8080/images"

	s3Adapter := &S3Adapter{
		client: s3Client,
		bucket: bucket,
	}

	// Create session store
	store := session.NewSessionStore()

	// Create handler
	captchaHandler := handler.NewCaptchaHandler(store, s3Adapter)
	captchaHandler.SetCloudfrontURL(cloudfrontURL)

	// Setup Echo
	e := echo.New()
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{http.MethodGet, http.MethodPost, http.MethodOptions},
		AllowHeaders:     []string{"*"},
		AllowCredentials: true,
	}))

	// Static files
	e.Static("/", "mock-frontend")

	// Debug endpoint to create session and set status
	e.POST("/api/debug/session", func(c echo.Context) error {
		user, sessionID := store.Create()
		user.Status = "registering"

		c.SetCookie(&http.Cookie{
			Name:     "session_id",
			Value:    sessionID,
			Path:     "/",
			HttpOnly: true,
		})

		return c.JSON(http.StatusOK, map[string]interface{}{
			"session_id": sessionID,
			"status":     user.Status,
		})
	})

	// CAPTCHA routes
	e.POST("/api/captcha/generate", captchaHandler.Generate)
	e.POST("/api/captcha/verify", captchaHandler.Verify)

	// Image proxy (serves S3 images locally for testing)
	e.GET("/images/*", func(c echo.Context) error {
		key := c.Param("*")
		data, err := s3Adapter.GetObject(key)
		if err != nil {
			return c.String(http.StatusNotFound, "Image not found")
		}
		return c.Blob(http.StatusOK, "image/png", data)
	})

	// Health check
	e.GET("/health", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	log.Println("Starting CAPTCHA test server on :8080")
	log.Println("Open http://localhost:8080/captcha-test.html")
	log.Fatal(e.Start(":8080"))
}
