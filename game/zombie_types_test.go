package game

import (
	"testing"

	"github.com/MorenoLand/Moreno.HalfBricked/engine/formats"
)

func TestSpawnRenderSizeScalesWithDeclaredSize(t *testing.T) {
	standard := nativeSpawnRenderSize(formats.Vec2{X: 29, Y: 31})
	if standard != nativeZombieDefaultRenderSize {
		t.Fatalf("standard zombie = %v, want default %v", standard, nativeZombieDefaultRenderSize)
	}
	if big := nativeSpawnRenderSize(formats.Vec2{X: 45}); big <= nativeSpawnRenderSize(formats.Vec2{X: 35}) || big <= standard {
		t.Fatalf("larger declared size must render larger: %v", big)
	}
	if nativeSpawnRenderSize(formats.Vec2{}) != nativeZombieDefaultRenderSize {
		t.Fatal("missing size keeps default")
	}
}
