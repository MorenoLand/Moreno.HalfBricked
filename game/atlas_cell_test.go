package game

import (
	"image"
	"testing"
)

func TestAtlasCellRectSplitsSheetHalves(t *testing.T) {
	if got, want := atlasCellRect("left", 4, 0, 5, 8, 1024, 1024), image.Rect(410, 0, 512, 128); got != want {
		t.Fatalf("left %v want %v", got, want)
	}
	if got, want := atlasCellRect("right", 0, 7, 5, 8, 1024, 1024), image.Rect(512, 896, 614, 1024); got != want {
		t.Fatalf("right %v want %v", got, want)
	}
	if got, want := atlasCellRect("", 1, 0, 5, 8, 1000, 800), barryCellRect(1, 0, 5, 8, 1000, 800); got != want {
		t.Fatalf("none %v want %v", got, want)
	}
}
