// Package captcha provides CAPTCHA generation for image-based verification.
package captcha

import (
	"image"
	"math/rand"
)

// Placement represents a character placement position.
type Placement struct {
	X      int
	Y      int
	Width  int
	Height int
}

// Bounds returns the image.Rectangle for this placement.
func (p Placement) Bounds() image.Rectangle {
	return image.Rect(p.X, p.Y, p.X+p.Width, p.Y+p.Height)
}

// Intersects checks if two placements overlap.
func (p Placement) Intersects(other Placement) bool {
	return p.Bounds().Overlaps(other.Bounds())
}

// PlacementManager manages non-overlapping character placements.
type PlacementManager struct {
	placements []Placement
	bgWidth    int
	bgHeight   int
	charWidth  int
	charHeight int
	maxRetries int
}

// NewPlacementManager creates a new placement manager.
func NewPlacementManager(bgWidth, bgHeight, charWidth, charHeight int) *PlacementManager {
	return &PlacementManager{
		placements: make([]Placement, 0),
		bgWidth:    bgWidth,
		bgHeight:   bgHeight,
		charWidth:  charWidth,
		charHeight: charHeight,
		maxRetries: 100,
	}
}

// TryPlace attempts to place a character at a random non-overlapping position.
// Returns the placement and success status.
func (pm *PlacementManager) TryPlace() (Placement, bool) {
	maxX := pm.bgWidth - pm.charWidth
	maxY := pm.bgHeight - pm.charHeight

	if maxX <= 0 || maxY <= 0 {
		return Placement{}, false
	}

	for retry := 0; retry < pm.maxRetries; retry++ {
		candidate := Placement{
			X:      rand.Intn(maxX),
			Y:      rand.Intn(maxY),
			Width:  pm.charWidth,
			Height: pm.charHeight,
		}

		if !pm.hasCollision(candidate) {
			pm.placements = append(pm.placements, candidate)
			return candidate, true
		}
	}

	return Placement{}, false
}

// hasCollision checks if a candidate placement overlaps with existing ones.
func (pm *PlacementManager) hasCollision(candidate Placement) bool {
	for _, existing := range pm.placements {
		if candidate.Intersects(existing) {
			return true
		}
	}
	return false
}

// PlacedCount returns the number of placed characters.
func (pm *PlacementManager) PlacedCount() int {
	return len(pm.placements)
}

// Reset clears all placements.
func (pm *PlacementManager) Reset() {
	pm.placements = pm.placements[:0]
}
