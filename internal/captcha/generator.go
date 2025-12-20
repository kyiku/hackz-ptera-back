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
	xdraw "golang.org/x/image/draw"
)

const (
	// CharacterSize is the size to resize characters to for CAPTCHA.
	CharacterSize = 50
	// DummiesPerType is the number of dummy characters per type.
	DummiesPerType = 30
)

// CharacterInfo holds information about a character image.
type CharacterInfo struct {
	Key   string      // S3 key (e.g., "character/char1.png")
	Image image.Image // Decoded and resized image
}

// GenerateResult holds the result of CAPTCHA generation.
type GenerateResult struct {
	Image          image.Image
	TargetX        int    // Target center X coordinate
	TargetY        int    // Target center Y coordinate
	TargetKey      string // Which character is the target (S3 key)
	TargetImageURL string // CloudFront URL for target character image
	TargetWidth    int
	TargetHeight   int
}

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
	keys, err := g.s3Client.ListObjects("static/backgrounds/")
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

// getCharacterImage retrieves a random character image from S3.
func (g *Generator) getCharacterImage() (image.Image, error) {
	keys, err := g.s3Client.ListObjects("static/character/")
	if err != nil {
		return nil, fmt.Errorf("failed to list characters: %w", err)
	}

	if len(keys) == 0 {
		return nil, fmt.Errorf("no character images found")
	}

	// Select random character
	key := keys[rand.Intn(len(keys))]

	data, err := g.s3Client.GetObject(key)
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
	key := "static/captcha/" + filename

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

// GenerateMultiCharacter creates a CAPTCHA with multiple characters.
// One random character is the target, the other 3 types are dummies (30 each).
func (g *Generator) GenerateMultiCharacter() (*GenerateResult, error) {
	// 1. Get background image
	bgImg, err := g.getRandomBackgroundImage()
	if err != nil {
		return nil, fmt.Errorf("failed to get background: %w", err)
	}

	// 2. Get all character images
	characters, err := g.getAllCharacterImages()
	if err != nil {
		return nil, fmt.Errorf("failed to get characters: %w", err)
	}

	if len(characters) < 4 {
		return nil, fmt.Errorf("need at least 4 character types, got %d", len(characters))
	}

	// 3. Select target (1 character) and dummies (remaining 3)
	targetIdx := rand.Intn(len(characters))
	target := characters[targetIdx]
	dummies := make([]CharacterInfo, 0, len(characters)-1)
	for i, c := range characters {
		if i != targetIdx {
			dummies = append(dummies, c)
		}
	}

	// 4. Initialize placement manager
	bgBounds := bgImg.Bounds()
	pm := NewPlacementManager(
		bgBounds.Dx(),
		bgBounds.Dy(),
		CharacterSize,
		CharacterSize,
	)

	// 5. Create result image
	result := image.NewRGBA(bgBounds)
	draw.Draw(result, bgBounds, bgImg, bgBounds.Min, draw.Src)

	// 6. Place dummies first (so target is drawn on top if overlap happens)
	for _, dummy := range dummies {
		for i := 0; i < DummiesPerType; i++ {
			placement, ok := pm.TryPlace()
			if !ok {
				// Can't place more, stop trying for this dummy type
				break
			}
			g.drawCharacter(result, dummy.Image, placement)
		}
	}

	// 7. Place target last
	targetPlacement, ok := pm.TryPlace()
	if !ok {
		return nil, fmt.Errorf("failed to place target character")
	}
	g.drawCharacter(result, target.Image, targetPlacement)

	// Calculate center coordinates for click detection
	centerX := targetPlacement.X + CharacterSize/2
	centerY := targetPlacement.Y + CharacterSize/2

	// Build target image URL
	targetImageURL := fmt.Sprintf("%s/%s", g.cloudfrontURL, target.Key)

	return &GenerateResult{
		Image:          result,
		TargetX:        centerX,
		TargetY:        centerY,
		TargetKey:      target.Key,
		TargetImageURL: targetImageURL,
		TargetWidth:    CharacterSize,
		TargetHeight:   CharacterSize,
	}, nil
}

// getAllCharacterImages retrieves all character images from S3 and resizes them.
func (g *Generator) getAllCharacterImages() ([]CharacterInfo, error) {
	keys, err := g.s3Client.ListObjects("static/character/")
	if err != nil {
		return nil, fmt.Errorf("failed to list characters: %w", err)
	}

	characters := make([]CharacterInfo, 0, len(keys))
	for _, key := range keys {
		data, err := g.s3Client.GetObject(key)
		if err != nil {
			return nil, fmt.Errorf("failed to get character %s: %w", key, err)
		}

		img, _, err := image.Decode(bytes.NewReader(data))
		if err != nil {
			return nil, fmt.Errorf("failed to decode character %s: %w", key, err)
		}

		// Resize to standard size
		resized := resizeImage(img, CharacterSize, CharacterSize)

		characters = append(characters, CharacterInfo{
			Key:   key,
			Image: resized,
		})
	}

	return characters, nil
}

// resizeImage resizes an image to the specified dimensions.
func resizeImage(src image.Image, width, height int) image.Image {
	dst := image.NewRGBA(image.Rect(0, 0, width, height))
	xdraw.CatmullRom.Scale(dst, dst.Bounds(), src, src.Bounds(), xdraw.Over, nil)
	return dst
}

// drawCharacter draws a character at the specified placement.
func (g *Generator) drawCharacter(dest *image.RGBA, char image.Image, p Placement) {
	draw.Draw(dest, p.Bounds(), char, char.Bounds().Min, draw.Over)
}
