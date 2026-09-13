package viewer

import (
	"testing"

	"github.com/MorenoLand/Moreno.HalfBricked/engine/formats"
)

func TestPropSourceBoundsPreserveReversedUVs(t *testing.T) {
	x0, y0, x1, y1 := propSourceBounds(64, 0, 1, 64, 128, 64)
	if x0 != 64 || y0 != 0 || x1 != 1 || y1 != 64 {
		t.Fatalf("reversed prop UVs = %.0f,%.0f -> %.0f,%.0f, want 64,0 -> 1,64", x0, y0, x1, y1)
	}
}

func TestPropSourceBoundsUsesTextureForDegenerateUVs(t *testing.T) {
	x0, y0, x1, y1 := propSourceBounds(0, 0, 0, 64, 128, 64)
	if x0 != 0 || y0 != 0 || x1 != 128 || y1 != 64 {
		t.Fatalf("degenerate prop UVs = %.0f,%.0f -> %.0f,%.0f, want 0,0 -> 128,64", x0, y0, x1, y1)
	}
}

func TestSetZoomReclampsCamera(t *testing.T) {
	viewer := &Viewer{Level: formats.Level{Width: 25, Height: 24}, TileSet: formats.TileSet{TileSize: 32}, Zoom: 1, CameraX: 224, CameraY: 287}
	viewer.SetZoom(.5)
	if viewer.CameraX != 0 || viewer.CameraY != 128 {
		t.Fatalf("camera after zoom = %.0f,%.0f, want 0,128", viewer.CameraX, viewer.CameraY)
	}
}
