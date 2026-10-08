package game

import (
	"github.com/MorenoLand/Moreno.HalfBricked/engine/weapons"
	"image/png"
	"math"
	"os"
	"testing"
)

func TestGrenadeExplosionNativeFrames(t *testing.T) {
	for frame := 0; frame < 9; frame++ {
		got, visible := weapons.GrenadeExplosionFrame((float64(frame) + .5) / 12)
		if !visible || got != frame {
			t.Fatalf("frame %d: got %d, visible %v", frame, got, visible)
		}
	}
	if frame, visible := weapons.GrenadeExplosionFrame(.75); !visible || frame != 0 {
		t.Fatal("native exact-duration frame must wrap before strict-greater cleanup")
	}
	for _, age := range []float64{-.01, .751, 1, math.NaN(), math.Inf(1)} {
		if _, visible := weapons.GrenadeExplosionFrame(age); visible {
			t.Fatalf("visible at age %g", age)
		}
	}
}
func TestGrenadeExplosionRetentionUsesNativeLifetime(t *testing.T) {
	p := &playState{explosions: []explosionState{{age: .2}}}
	p.updateExplosions()
	if len(p.explosions) != 1 {
		t.Fatal("explosion removed at old 200ms lifetime")
	}
	p.explosions[0].age = .74
	p.updateExplosions()
	if len(p.explosions) != 0 {
		t.Fatal("explosion survived native 750ms lifetime")
	}
}
func TestGrenadeExplosionNativeUVAndProjection(t *testing.T) {
	for frame := 0; frame < 9; frame++ {
		quad, visible := weapons.GrenadeExplosionVertices((float64(frame)+.5)/12, 240, 160, 1, 2, 1.6875, 512, 128)
		if !visible {
			t.Fatal("missing quad")
		}
		wantU := float32(frame) * float32(.11) * 512
		if quad[0].SrcX != wantU || math.Abs(float64(quad[1].SrcX-quad[0].SrcX)-56.32) > .0001 {
			t.Fatalf("frame %d UV = %g..%g", frame, quad[0].SrcX, quad[1].SrcX)
		}
		if quad[0].SrcY != 0 || quad[2].SrcY != 128 {
			t.Fatal("PNG vertically flipped")
		}
		if quad[1].DstX-quad[0].DstX != 240 || quad[2].DstY-quad[0].DstY != 480 {
			t.Fatal("nonuniform dimensions")
		}
		if (quad[0].DstX+quad[1].DstX)/2 != 480 || (quad[0].DstY+quad[2].DstY)/2 != 130 {
			t.Fatal("incorrect center or lift")
		}
	}
}
func TestGrenadeExplosionOriginalSheetDimensions(t *testing.T) {
	file, err := os.Open("bin/data-cache/textures/Common0/Textures/explosion2_SD.png")
	if os.IsNotExist(err) {
		t.Skip("original content not installed")
	}
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	config, err := png.DecodeConfig(file)
	if err != nil {
		t.Fatal(err)
	}
	if config.Width != 512 || config.Height != 128 {
		t.Fatalf("sheet %dx%d, want 512x128", config.Width, config.Height)
	}
}
