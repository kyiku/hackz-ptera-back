// Package util provides utility functions.
package util

import (
	"strings"
	"unicode"
)

// hiraganaStart is the start of the hiragana Unicode block.
const hiraganaStart = 0x3040

// hiraganaEnd is the end of the hiragana Unicode block.
const hiraganaEnd = 0x309F

// katakanaStart is the start of the katakana Unicode block.
const katakanaStart = 0x30A0

// katakanaEnd is the end of the katakana Unicode block.
const katakanaEnd = 0x30FF

// kanaOffset is the offset between hiragana and katakana.
const kanaOffset = katakanaStart - hiraganaStart

// HiraganaToKatakana converts all hiragana characters to katakana.
func HiraganaToKatakana(s string) string {
	var result strings.Builder
	for _, r := range s {
		if r >= hiraganaStart && r <= hiraganaEnd {
			result.WriteRune(r + kanaOffset)
		} else {
			result.WriteRune(r)
		}
	}
	return result.String()
}

// KatakanaToHiragana converts all katakana characters to hiragana.
func KatakanaToHiragana(s string) string {
	var result strings.Builder
	for _, r := range s {
		if r >= katakanaStart && r <= katakanaEnd {
			result.WriteRune(r - kanaOffset)
		} else {
			result.WriteRune(r)
		}
	}
	return result.String()
}

// NormalizeForComparison normalizes a string for comparison.
// It converts hiragana to katakana and trims whitespace.
func NormalizeForComparison(s string) string {
	// Trim whitespace (including full-width space)
	s = strings.TrimSpace(s)
	s = strings.Trim(s, "\u3000") // Full-width space

	// Convert to katakana
	return HiraganaToKatakana(s)
}

// KanaMatch checks if two strings match, ignoring hiragana/katakana differences.
func KanaMatch(input, answer string) bool {
	// Handle empty strings
	if input == "" && answer == "" {
		return true
	}

	// Normalize both strings
	normalizedInput := NormalizeForComparison(input)
	normalizedAnswer := NormalizeForComparison(answer)

	return normalizedInput == normalizedAnswer
}

// IsHiragana checks if a rune is a hiragana character.
func IsHiragana(r rune) bool {
	return r >= hiraganaStart && r <= hiraganaEnd
}

// IsKatakana checks if a rune is a katakana character.
func IsKatakana(r rune) bool {
	return r >= katakanaStart && r <= katakanaEnd
}

// IsKana checks if a rune is either hiragana or katakana.
func IsKana(r rune) bool {
	return IsHiragana(r) || IsKatakana(r)
}

// ContainsOnlyKana checks if a string contains only kana characters.
func ContainsOnlyKana(s string) bool {
	for _, r := range s {
		if !IsKana(r) && !unicode.IsSpace(r) {
			return false
		}
	}
	return true
}
