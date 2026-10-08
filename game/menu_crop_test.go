package game

import "testing"

func TestQuitLabelIncludesOriginalAtlasGlyph(t *testing.T) {
	rect, ok := buttonTextRect(6)
	if !ok || rect.Min.Y > 94 || rect.Max.Y < 110 {
		t.Fatalf("QUIT crop %v truncates original atlas glyph", rect)
	}
}
