// Package storage provides S3 storage integration.
package storage

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/png"
	"math/rand"
	"strings"

	"github.com/google/uuid"
)

// S3ClientInterface defines the interface for S3 operations.
type S3ClientInterface interface {
	GetObject(key string) ([]byte, error)
	PutObject(key string, data []byte) error
	ListObjects(prefix string) ([]string, error)
}

// S3Client wraps S3 operations for image storage.
type S3Client struct {
	client        S3ClientInterface
	bucket        string
	cloudfrontURL string
}

// NewS3Client creates a new S3Client.
func NewS3Client(client S3ClientInterface, bucket string, cloudfrontURL string) *S3Client {
	return &S3Client{
		client:        client,
		bucket:        bucket,
		cloudfrontURL: strings.TrimSuffix(cloudfrontURL, "/"),
	}
}

// GetRandomBackgroundImage returns a random background image.
func (c *S3Client) GetRandomBackgroundImage() (image.Image, error) {
	keys, err := c.client.ListObjects("backgrounds/")
	if err != nil {
		return nil, fmt.Errorf("failed to list background images: %w", err)
	}

	if len(keys) == 0 {
		return nil, errors.New("no background images available")
	}

	// Select random background
	randomKey := keys[rand.Intn(len(keys))]

	data, err := c.client.GetObject(randomKey)
	if err != nil {
		return nil, fmt.Errorf("failed to get background image: %w", err)
	}

	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed to decode background image: %w", err)
	}

	return img, nil
}

// GetCharacterImage returns the character image.
func (c *S3Client) GetCharacterImage() (image.Image, error) {
	data, err := c.client.GetObject("character/char.png")
	if err != nil {
		return nil, fmt.Errorf("failed to get character image: %w", err)
	}

	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed to decode character image: %w", err)
	}

	return img, nil
}

// GetFishImageURL returns the CloudFront URL for a fish image.
func (c *S3Client) GetFishImageURL(fishName string) (string, error) {
	return fmt.Sprintf("%s/fish/%s.jpg", c.cloudfrontURL, fishName), nil
}

// UploadCaptchaImage uploads a captcha image and returns its CloudFront URL.
func (c *S3Client) UploadCaptchaImage(img image.Image) (string, error) {
	// Generate unique filename
	filename := uuid.New().String() + ".png"
	key := "captcha/" + filename

	// Encode image to PNG
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return "", fmt.Errorf("failed to encode captcha image: %w", err)
	}

	// Upload to S3
	if err := c.client.PutObject(key, buf.Bytes()); err != nil {
		return "", fmt.Errorf("failed to upload captcha image: %w", err)
	}

	url := fmt.Sprintf("%s/%s", c.cloudfrontURL, key)
	return url, nil
}

// ListFishImages returns a list of fish names available in storage.
func (c *S3Client) ListFishImages() ([]string, error) {
	keys, err := c.client.ListObjects("fish/")
	if err != nil {
		return nil, fmt.Errorf("failed to list fish images: %w", err)
	}

	fishNames := make([]string, 0, len(keys))
	for _, key := range keys {
		// Extract fish name from key (e.g., "fish/onikamasu.jpg" -> "onikamasu")
		name := strings.TrimPrefix(key, "fish/")
		name = strings.TrimSuffix(name, ".jpg")
		name = strings.TrimSuffix(name, ".png")
		fishNames = append(fishNames, name)
	}

	return fishNames, nil
}
