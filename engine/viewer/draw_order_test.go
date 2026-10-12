package viewer

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MorenoLand/Moreno.HalfBricked/engine/formats"
	"github.com/hajimehoshi/ebiten/v2"
)

// These tests encode Research/native/prop-entity-draw-order-2026-10-12.md: H rows, props and entities go through one
// y-bucket list (bucket = clamp(int((y-camY)/(2z)+140), 0, 349), z = 0.65 -> 1.3 px per bucket); within a bucket the
// order is hb (before bucket 10), rows, props, entities.

type blankTextures struct{}

func (blankTextures) Texture(string) (*ebiten.Image, error) { return ebiten.NewImage(8, 8), nil }

func loadLevel(t *testing.T, name string) formats.Level {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "bin", "data-cache", "levels", name+".json"))
	if err != nil {
		t.Skipf("level data not available: %v", err)
	}
	var level formats.Level
	if err := json.Unmarshal(data, &level); err != nil {
		t.Fatal(err)
	}
	return level
}

// drawTraceFor draws the level with the camera centred on (centreX, centreY) at the native default zoom and returns
// the draw steps in order, with one "entity:<y>" step for every probe entity (placed by EntityBand.Contains, like the
// game does) and "first" for the ground slice.
func drawTraceFor(t *testing.T, level formats.Level, centreX, centreY float64, probes []float64) []string {
	t.Helper()
	v := &Viewer{Level: level, Textures: blankTextures{}, Zoom: 1, RenderScaleX: 1, RenderScaleY: 1, Props: true,
		Layers: map[formats.LayerKind]bool{formats.LayerG: true, formats.LayerD: true, formats.LayerH: true, formats.LayerHB: true}}
	v.CameraX, v.CameraY = centreX-240, centreY-160
	var trace []string
	v.drawTrace = func(label string) {
		if !strings.HasPrefix(label, "band:") {
			trace = append(trace, label)
		}
	}
	screen := ebiten.NewImage(480, 320)
	v.DrawWithEntityBands(screen, func(_ *ebiten.Image, band EntityBand) {
		if band.First {
			trace = append(trace, "first")
		}
		for _, y := range probes {
			if band.Contains(y) {
				trace = append(trace, fmt.Sprintf("entity:%g", y))
			}
		}
	})
	return trace
}

func indexOf(t *testing.T, trace []string, label string) int {
	t.Helper()
	for index, entry := range trace {
		if entry == label {
			return index
		}
	}
	t.Fatalf("%q not in draw trace %v", label, trace)
	return -1
}

func TestDepthBucketTable(t *testing.T) {
	cases := []struct {
		y    float64
		want int
	}{{288, 140}, {319.0, 163}, {319.5, 164}, {320.25, 164}, {321.0, 165}, {-1000, 0}, {5000, 349}}
	for _, c := range cases {
		if got := DepthBucket(c.y, 288, 0.65); got != c.want {
			t.Errorf("DepthBucket(%v, 288, 0.65) = %d, want %d", c.y, got, c.want)
		}
	}
	if hRowKey(8) != 320.25 {
		t.Fatalf("hRowKey(8) = %v, want 320.25", hRowKey(8))
	}
}

func TestPropLiftAndKeyTruncate(t *testing.T) {
	for _, c := range []struct{ height, lift float64 }{{3.5, 96}, {2.5, 64}, {2.75, 64}, {0.5, 0}, {1.5, 32}, {8, 256}} {
		if got := propLift(c.height); got != c.lift {
			t.Errorf("propLift(%v) = %v, want %v", c.height, got, c.lift)
		}
	}
	if propKeyY(397.9) != 397 || propKeyY(-0.5) != 0 {
		t.Fatalf("propKeyY must truncate toward zero: %v %v", propKeyY(397.9), propKeyY(-0.5))
	}
	for name, texture := range map[string]string{"World5Level1": "Prop_wbuilding1", "World1Level0": "Prop_building1", "World2Level0": "pot_small"} {
		v := &Viewer{Level: loadLevel(t, name)}
		found := false
		for _, prop := range v.levelProps() {
			if prop.texture == texture && prop.height != float64(int(prop.height)) {
				found = true
				if lift := propLift(prop.height); lift != float64(int(prop.height))*32 || lift > prop.height*32 {
					t.Errorf("%s %s height %v lift %v", name, texture, prop.height, lift)
				}
			}
		}
		if !found {
			t.Errorf("%s has no fractional-height %s", name, texture)
		}
	}
}

// The hydrant is tile 232 in the h layer over tile 248 in hb: an entity below the bottom edge of the row under the top
// tile is drawn over it, one behind is drawn under it.
func TestHydrantTopTileIsOverEntitiesBehindIt(t *testing.T) {
	level := loadLevel(t, "World1Level0")
	if level.Layers[formats.LayerH][8*level.Width+43]&0xffff != 232 || level.Layers[formats.LayerHB][9*level.Width+43]&0xffff != 248 {
		t.Fatal("World1Level0 hydrant tiles are not at col 43 rows 8/9")
	}
	probes := []float64{319.0, 319.5, 320.0, 321.0, 330.0}
	trace := drawTraceFor(t, level, 43*32+16, 288, probes)
	row := indexOf(t, trace, "row:8")
	if indexOf(t, trace, "entity:319") > row {
		t.Errorf("entity at y 319.0 must be under the hydrant top row: %v", trace)
	}
	for _, y := range probes[1:] {
		if indexOf(t, trace, fmt.Sprintf("entity:%g", y)) < row {
			t.Errorf("entity at y %v must be over the hydrant top row: %v", y, trace)
		}
	}
	// 319.5 < 320.25, but both are bucket 164: the entity of the same bucket is drawn after the row.
	if DepthBucket(319.5, 288, 0.65) != DepthBucket(hRowKey(8), 288, 0.65) {
		t.Error("319.5 and the row key 320.25 must share a bucket")
	}
}

func TestHydrantTopTileWorld1Level1(t *testing.T) {
	level := loadLevel(t, "World1Level1")
	if level.Layers[formats.LayerH][2*level.Width+29]&0xffff != 232 || level.Layers[formats.LayerHB][3*level.Width+29]&0xffff != 248 {
		t.Fatal("World1Level1 hydrant tiles are not at col 29 rows 2/3")
	}
	// row 2: key (2+2)*32+0.25 = 128.25; camera centred on the row below it.
	trace := drawTraceFor(t, level, 29*32+16, 96, []float64{127, 128.5, 140})
	row := indexOf(t, trace, "row:2")
	if indexOf(t, trace, "entity:127") > row || indexOf(t, trace, "entity:128.5") < row || indexOf(t, trace, "entity:140") < row {
		t.Errorf("entities around the World1Level1 hydrant row are misordered: %v", trace)
	}
}

// No blacklist and no height test: every prop is keyed by its y (props do not pass over entities because they are tall).
func TestTallPropsAreOrderedByYAgainstEntities(t *testing.T) {
	cases := []struct {
		level, texture string
		x, y           float64
		under, over    float64
		camUp          float64
	}{
		{"World3Level0", "tower", 832, 704, 700, 720, 0},
		{"World3Level2", "blossom", 976, 352, 340, 360, 0},
		{"World0Level0", "tree_01", 182, 397, 380, 400, 100},
	}
	for _, c := range cases {
		// the camera sits above a tall prop so that its lifted quad is on screen
		trace := drawTraceFor(t, loadLevel(t, c.level), c.x, c.y-c.camUp, []float64{c.under, c.over})
		prop := indexOf(t, trace, "prop:"+c.texture)
		if indexOf(t, trace, fmt.Sprintf("entity:%g", c.under)) > prop {
			t.Errorf("%s: entity at y %v must be under the prop: %v", c.level, c.under, trace)
		}
		if indexOf(t, trace, fmt.Sprintf("entity:%g", c.over)) < prop {
			t.Errorf("%s: entity at y %v must be over the prop: %v", c.level, c.over, trace)
		}
	}
}

// Props in one bucket keep list order and are not re-sorted by exact y (the old sort was by y).
func TestPropsKeepListOrderInsideABucket(t *testing.T) {
	level := formats.Level{Width: 4, Height: 4, Layers: map[formats.LayerKind][]uint32{formats.LayerG: make([]uint32, 16), formats.LayerD: make([]uint32, 16), formats.LayerHB: make([]uint32, 16), formats.LayerH: emptyWords(16)},
		Props: []formats.Prop{{Texture: "second_y", X: 64, Y: 101, Height: 1, ScaleX: 1, ScaleY: 1}, {Texture: "first_y", X: 64, Y: 100, Height: 1, ScaleX: 1, ScaleY: 1}}}
	trace := drawTraceFor(t, level, 64, 100, nil)
	if indexOf(t, trace, "prop:second_y") > indexOf(t, trace, "prop:first_y") {
		t.Errorf("props of one bucket must keep list order: %v", trace)
	}
}

func TestHBIsDrawnAfterDAndAfterTheFirstBand(t *testing.T) {
	level := loadLevel(t, "World0Level2")
	both := 0
	for index, d := range level.Layers[formats.LayerD] {
		if d != 0 && int32(d) >= 0 && level.Layers[formats.LayerHB][index] != 0 && int32(level.Layers[formats.LayerHB][index]) >= 0 {
			both++
		}
	}
	if both == 0 {
		t.Fatal("World0Level2 should have cells with both hb and d")
	}
	trace := drawTraceFor(t, level, 600, 400, nil)
	d, first, hb := indexOf(t, trace, "layer:d"), indexOf(t, trace, "first"), indexOf(t, trace, "hb")
	if !(indexOf(t, trace, "layer:g") < d && d < first && first < hb) {
		t.Errorf("want g, d, first band, hb: %v", trace)
	}
}

func TestHRowsAreGatedByTilesAndCamera(t *testing.T) {
	for _, step := range drawTraceFor(t, loadLevel(t, "World0Level0"), 300, 300, nil) {
		if strings.HasPrefix(step, "row:") {
			t.Errorf("World0Level0 has no h tiles but drew %s", step)
		}
	}
	level := loadLevel(t, "World1Level0")
	trace := drawTraceFor(t, level, 43*32+16, 288, nil)
	rows := 0
	for _, step := range trace {
		if strings.HasPrefix(step, "row:") {
			rows++
			var row int
			fmt.Sscanf(step, "row:%d", &row)
			if row < 2 || row > 16 { // native margin: (top>>5)-2 .. ((bottom+10)>>5)+3
				t.Errorf("row %d is outside the camera but was submitted", row)
			}
		}
	}
	if rows == 0 {
		t.Error("World1Level0 near the hydrant must submit h rows")
	}
}

func emptyWords(n int) []uint32 {
	words := make([]uint32, n)
	for index := range words {
		words[index] = 0xffffffff
	}
	return words
}
