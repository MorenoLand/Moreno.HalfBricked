package main

import (
	"image"
	"math"
	"testing"
)

func TestZombieRenderPreservesNativeSquareDimensions(t *testing.T) {
	source := image.Rect(0, 0, 51, 64)
	x, y := zombieSpriteScale(48, 48, source, 1, 2, 1.6875)
	width, height := float64(source.Dx())*x*2, float64(source.Dy())*y*1.6875
	if math.Abs(width-96) > .00001 || math.Abs(height-96) > .00001 {
		t.Fatalf("zombie dimensions = %gx%g, want native uniform 96x96", width, height)
	}
}
