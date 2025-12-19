// Package captcha provides CAPTCHA generation for image-based verification.
package captcha

import (
	"bytes"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"math/rand"

	"github.com/google/uuid"
)

// S3ClientInterface defines the interface for S3 operations.
type S3ClientInterface interface {
	GetObject(key string) ([]byte, error)
	PutObject(key string, data []byte) error
	ListObjects(prefix string) ([]string, error)
}

// Generator generates CAPTCHA images.
type Generator struct {
	s3Client      S3ClientInterface
	cloudfrontURL string
}

// NewGenerator creates a new CAPTCHA generator.
func NewGenerator(s3Client S3ClientInterface, cloudfrontURL string) *Generator {
	return &Generator{
		s3Client:      s3Client,
		cloudfrontURL: cloudfrontURL,
	}
}

// Generate creates a new CAPTCHA image with a hidden character.
// Returns the composed image, character X position, character Y position, and error.
func (g *Generator) Generate() (image.Image, int, int, error) {
	// Get random background image
	bgImg, err := g.getRandomBackgroundImage()
	if err != nil {
		return nil, 0, 0, fmt.Errorf("failed to get background: %w", err)
	}

	// Get character image
	charImg, err := g.getCharacterImage()
	if err != nil {
		return nil, 0, 0, fmt.Errorf("failed to get character: %w", err)
	}

	// Calculate random position for character
	bgBounds := bgImg.Bounds()
	charBounds := charImg.Bounds()

	maxX := bgBounds.Dx() - charBounds.Dx()
	maxY := bgBounds.Dy() - charBounds.Dy()

	if maxX <= 0 {
		maxX = 1
	}
	if maxY <= 0 {
		maxY = 1
	}

	targetX := rand.Intn(maxX)
	targetY := rand.Intn(maxY)

	// Compose the image
	result := g.Compose(bgImg, charImg, targetX, targetY)

	return result, targetX, targetY, nil
}

// getRandomBackgroundImage retrieves a random background image from S3.
func (g *Generator) getRandomBackgroundImage() (image.Image, error) {
	keys, err := g.s3Client.ListObjects("backgrounds/")
	if err != nil {
		return nil, fmt.Errorf("failed to list backgrounds: %w", err)
	}

	if len(keys) == 0 {
		return nil, fmt.Errorf("no background images found")
	}

	// Select random background
	key := keys[rand.Intn(len(keys))]

	data, err := g.s3Client.GetObject(key)
	if err != nil {
		return nil, fmt.Errorf("failed to get background image: %w", err)
	}

	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed to decode background image: %w", err)
	}

	return img, nil
}

// getCharacterImage retrieves the character image from S3.
func (g *Generator) getCharacterImage() (image.Image, error) {
	data, err := g.s3Client.GetObject("character/char.png")
	if err != nil {
		return nil, fmt.Errorf("failed to get character image: %w", err)
	}

	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed to decode character image: %w", err)
	}

	return img, nil
}

// Compose overlays the character image onto the background at the specified position.
func (g *Generator) Compose(bg image.Image, char image.Image, x, y int) image.Image {
	bgBounds := bg.Bounds()
	result := image.NewRGBA(bgBounds)

	// Draw background
	draw.Draw(result, bgBounds, bg, bgBounds.Min, draw.Src)

	// Draw character at specified position
	charBounds := char.Bounds()
	destRect := image.Rect(x, y, x+charBounds.Dx(), y+charBounds.Dy())
	draw.Draw(result, destRect, char, charBounds.Min, draw.Over)

	return result
}

// Upload uploads the CAPTCHA image to S3 and returns the CloudFront URL.
func (g *Generator) Upload(img image.Image) (string, error) {
	// Generate unique filename
	filename := uuid.New().String() + ".png"
	key := "captcha/" + filename

	// Encode image to PNG
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return "", fmt.Errorf("failed to encode image: %w", err)
	}

	// Upload to S3
	if err := g.s3Client.PutObject(key, buf.Bytes()); err != nil {
		return "", fmt.Errorf("failed to upload image: %w", err)
	}

	url := fmt.Sprintf("%s/%s", g.cloudfrontURL, key)
	return url, nil
}
