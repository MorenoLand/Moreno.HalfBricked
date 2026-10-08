package viewer

import (
	"testing"

	"github.com/MorenoLand/Moreno.HalfBricked/engine/formats"
	"github.com/hajimehoshi/ebiten/v2"
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

func TestAtlasTileVerticesUseFullNearestTexelBounds(t *testing.T) {
	viewer := &Viewer{Atlas: ebiten.NewImage(512, 512), TileSet: formats.TileSet{TileSize: 32, UVOffset: .6}}
	vertices, ok := viewer.atlasTileVertices(17, 0, 0, 32)
	if !ok || vertices[0].SrcX != 32 || vertices[0].SrcY != 32 || vertices[3].SrcX != 64 || vertices[3].SrcY != 64 {
		t.Fatalf("nearest tile bounds = (%v,%v)-(%v,%v), want (32,32)-(64,64)", vertices[0].SrcX, vertices[0].SrcY, vertices[3].SrcX, vertices[3].SrcY)
	}
}
func TestAtlasTileVerticesApplyNativeFlipBits(t *testing.T) {
	viewer := &Viewer{Atlas: ebiten.NewImage(512, 512), TileSet: formats.TileSet{TileSize: 32}}
	vertices, ok := viewer.atlasTileVertices(17|0x00030000, 0, 0, 32)
	if !ok || vertices[0].SrcX != 64 || vertices[1].SrcX != 32 || vertices[0].SrcY != 64 || vertices[2].SrcY != 32 {
		t.Fatalf("flipped tile source = (%v,%v)-(%v,%v), want x 64->32 and y 64->32", vertices[0].SrcX, vertices[1].SrcX, vertices[0].SrcY, vertices[2].SrcY)
	}
}
func TestHDAtlasUsesSourcePixelsAndNativeWorldCells(t *testing.T) {
	v := &Viewer{Atlas: ebiten.NewImage(512, 512), Level: formats.Level{Width: 25, Height: 24}, TileSet: formats.TileSet{TileSize: 64, TileShift: 6}, Zoom: 1}
	vertices, ok := v.atlasTileVertices(9, 6, 7, v.tileSize())
	if !ok || vertices[0].SrcX != 64 || vertices[0].SrcY != 64 || vertices[3].SrcX != 128 || vertices[3].SrcY != 128 {
		t.Fatalf("HD source crop: %v", vertices)
	}
	if vertices[0].DstX != 192 || vertices[0].DstY != 224 || vertices[3].DstX != 224 || vertices[3].DstY != 256 || v.Level.Width*v.tileSize() != 800 || v.Level.Height*v.tileSize() != 768 {
		t.Fatalf("native world grid: %v", vertices)
	}
}

func TestNeighbouringTilesShareTheirEdgeAtAnyCameraAndZoom(t *testing.T) {
	v := &Viewer{Atlas: ebiten.NewImage(64, 64), Zoom: 1}
	for _, zoom := range []float64{.45, .5, .65, 1, 1.37} {
		for _, camera := range []float64{0, 3.3, 17.77, 101.123} {
			v.Zoom, v.CameraX, v.CameraY = zoom, camera, camera*.7
			for x := 0; x < 20; x++ {
				a, okA := v.atlasTileVertices(1, x, 4, 32)
				b, okB := v.atlasTileVertices(1, x+1, 4, 32)
				c, _ := v.atlasTileVertices(1, x, 5, 32)
				if !okA || !okB {
					t.Skip("atlas too small for the test tile")
				}
				if a[1].DstX != b[0].DstX || a[3].DstY != c[0].DstY {
					t.Fatalf("zoom %v camera %v tile %d: edge %v != %v or %v != %v", zoom, camera, x, a[1].DstX, b[0].DstX, a[3].DstY, c[0].DstY)
				}
			}
		}
	}
}
