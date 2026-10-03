package viewer

import (
	"github.com/MorenoLand/Moreno.HalfBricked/engine/formats"
	"github.com/hajimehoshi/ebiten/v2"
	"math"
	"testing"
)

func TestRenderLayerNativeEmptyWords(t *testing.T) {
	for _, test := range []struct {
		kind formats.LayerKind
		word uint32
		draw bool
	}{{formats.LayerG, math.MaxUint32, true}, {formats.LayerG, 0, true}, {formats.LayerHB, 0, false}, {formats.LayerD, 0, false}, {formats.LayerH, math.MaxUint32, false}, {formats.LayerHB, 0x10000, true}} {
		if got := rendersTileWord(test.kind, test.word); got != test.draw {
			t.Fatalf("layer %s word %08x draws=%v, want %v", test.kind, test.word, got, test.draw)
		}
	}
}

func TestAtlasGroundSentinelRepeatsNativeTexture(t *testing.T) {
	v := &Viewer{Atlas: ebiten.NewImage(512, 512), Zoom: 1}
	vertices, ok := v.atlasTileVertices(math.MaxUint32, 26, 0, 32)
	if !ok || vertices[0].SrcX != 512 || vertices[0].SrcY != 512 || vertices[3].SrcX != 480 || vertices[3].SrcY != 480 {
		t.Fatalf("ground sentinel UVs = %+v, valid=%v", vertices, ok)
	}
	if vertices[0].DstX != 832 || vertices[3].DstX != 864 || vertices[0].DstY != 0 || vertices[3].DstY != 32 {
		t.Fatalf("ground sentinel geometry = %+v", vertices)
	}
}

func TestAtlasTileRepeatRetainsFlipBits(t *testing.T) {
	v := &Viewer{Atlas: ebiten.NewImage(512, 512), Zoom: 1}
	vertices, ok := v.atlasTileVertices(273|0x10000, 0, 0, 32)
	if !ok || vertices[0].SrcX != 64 || vertices[1].SrcX != 32 || vertices[0].SrcY != 32 || vertices[2].SrcY != 64 {
		t.Fatalf("repeated tile UVs = %+v, valid=%v", vertices, ok)
	}
}
