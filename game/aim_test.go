package game

import (
	"math"
	"testing"
)

// A mouse shot leaves the muzzle for the cursor itself: the bullet line crosses the aim ray at the cursor's own
// distance, so the error no longer grows with it.
func TestMouseShotsAimAtTheCursor(t *testing.T) {
	p := &playState{}
	for _, distance := range []float64{60, 150, 400} {
		p.aimPointDistance = distance
		offsetX, offsetY := 6.0, 24.0 // a muzzle off to the side and below the body
		dx, dy := p.projectileDirection(1, 0, offsetX, offsetY)
		// From the muzzle, travelling to the line x = distance, the shot's y must come back to the aim ray (y = 0).
		travel := (distance - offsetX) / dx
		if got := offsetY + dy*travel; math.Abs(got) > 1e-6 {
			t.Fatalf("cursor at %v: the shot is %.2f px off the aim ray there", distance, got)
		}
	}
}
