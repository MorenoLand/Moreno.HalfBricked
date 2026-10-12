package game

import (
	"testing"

	"github.com/MorenoLand/Moreno.HalfBricked/engine/viewer"
)

// Research/native/prop-entity-draw-order-2026-10-12.md: entities are keyed by the bucket of their raw position y (no
// feet offset); an entity in the bucket of a separator is drawn after it, so the band after a row keyed 320.25
// (bucket 164 at camY 288, z 0.65) starts at bucket 164.
func TestInDepthBandUsesRawPositionBuckets(t *testing.T) {
	a := &app{depthBandActive: true, depthBand: viewer.EntityBand{LoBucket: 164, HiBucket: 200, CamY: 288, Z: 0.65}}
	for _, c := range []struct {
		y    float64
		want bool
	}{{319.0, false}, {319.5, true}, {320.25, true}, {330, true}, {-1000, false}} {
		if got := a.inDepthBand(c.y); got != c.want {
			t.Errorf("inDepthBand(%v) = %v, want %v (bucket %d)", c.y, got, c.want, a.depthBucket(c.y))
		}
	}
	a.depthBandActive = false
	if !a.inDepthBand(-1e9) {
		t.Error("outside a banded draw every entity is in the band")
	}
}
