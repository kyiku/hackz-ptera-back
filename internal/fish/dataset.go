// Package fish provides fish dataset management for OTP verification.
package fish

import (
	"errors"
	"math/rand"
)

// Fish represents a fish in the dataset.
type Fish struct {
	Name     string // Fish name in Japanese (katakana)
	Filename string // Image filename
}

// predefinedFish contains the list of fish available for OTP.
var predefinedFish = []Fish{
	{Name: "オニカマス", Filename: "onikamasu.jpg"},
	{Name: "ホウボウ", Filename: "houbou.jpg"},
	{Name: "マツカサウオ", Filename: "matsukasauo.jpg"},
	{Name: "ハリセンボン", Filename: "harisenbon.jpg"},
	{Name: "カワハギ", Filename: "kawahagi.jpg"},
	{Name: "フグ", Filename: "fugu.jpg"},
	{Name: "タツノオトシゴ", Filename: "tatsunootoshigo.jpg"},
	{Name: "オコゼ", Filename: "okoze.jpg"},
	{Name: "アンコウ", Filename: "ankou.jpg"},
	{Name: "ウツボ", Filename: "utsubo.jpg"},
	{Name: "ハモ", Filename: "hamo.jpg"},
	{Name: "カサゴ", Filename: "kasago.jpg"},
	{Name: "メバル", Filename: "mebaru.jpg"},
	{Name: "アイナメ", Filename: "ainame.jpg"},
	{Name: "カレイ", Filename: "karei.jpg"},
	{Name: "ヒラメ", Filename: "hirame.jpg"},
	{Name: "タイ", Filename: "tai.jpg"},
	{Name: "スズキ", Filename: "suzuki.jpg"},
	{Name: "アジ", Filename: "aji.jpg"},
	{Name: "サバ", Filename: "saba.jpg"},
}

// Dataset manages the fish dataset.
type Dataset struct {
	fish []Fish
}

// NewDataset creates a new fish dataset.
func NewDataset() *Dataset {
	return &Dataset{
		fish: predefinedFish,
	}
}

// GetRandom returns a random fish from the dataset.
func (d *Dataset) GetRandom() (*Fish, error) {
	if len(d.fish) == 0 {
		return nil, errors.New("no fish available")
	}
	idx := rand.Intn(len(d.fish))
	fish := d.fish[idx]
	return &fish, nil
}

// GetRandomExcluding returns a random fish excluding the specified names.
func (d *Dataset) GetRandomExcluding(excluded []string) (*Fish, error) {
	excludeMap := make(map[string]bool)
	for _, name := range excluded {
		excludeMap[name] = true
	}

	available := make([]Fish, 0)
	for _, f := range d.fish {
		if !excludeMap[f.Name] {
			available = append(available, f)
		}
	}

	if len(available) == 0 {
		return nil, errors.New("no fish available after exclusion")
	}

	idx := rand.Intn(len(available))
	fish := available[idx]
	return &fish, nil
}

// Count returns the number of fish in the dataset.
func (d *Dataset) Count() int {
	return len(d.fish)
}

// GetByName returns a fish by its name.
func (d *Dataset) GetByName(name string) (*Fish, error) {
	for _, f := range d.fish {
		if f.Name == name {
			return &f, nil
		}
	}
	return nil, errors.New("fish not found: " + name)
}

// ListAll returns all fish in the dataset.
func (d *Dataset) ListAll() []*Fish {
	result := make([]*Fish, len(d.fish))
	for i := range d.fish {
		result[i] = &d.fish[i]
	}
	return result
}
