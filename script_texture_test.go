package main

import (
	"fmt"
	"image"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MorenoLand/Moreno.HalfBricked/engine/formats"
	"github.com/MorenoLand/Moreno.HalfBricked/engine/scripting"
)

func TestTutorialTextureGeometry(t *testing.T) {
	testTutorialTextureGeometry(t, image.Rect(0, 0, 128, 256), "", "Tutorial_Image_SD")
}
func TestTutorialTextureOriginalAssets(t *testing.T) {
	script, err := os.ReadFile("bin/data-cache/source/World0/Scripts/world0_level0_entry.script")
	if os.IsNotExist(err) {
		t.Skip("original script cache is unavailable")
	}
	if err != nil {
		t.Fatal(err)
	}
	declarations := strings.Join(strings.Fields(string(script)), "")
	for _, asset := range []string{"Tutorial_Image_SD", "Tutorial_Xperia_Image"} {
		t.Run(asset, func(t *testing.T) {
			file, err := os.Open(filepath.Join("bin", "data-cache", "textures", "Common0", "Textures", asset+".png"))
			if os.IsNotExist(err) {
				t.Skip("original texture cache is unavailable")
			}
			if err != nil {
				t.Fatal(err)
			}
			bitmap, err := png.Decode(file)
			file.Close()
			if err != nil {
				t.Fatal(err)
			}
			if bitmap.Bounds() != image.Rect(0, 0, 128, 256) {
				t.Fatalf("%s bounds = %v", asset, bitmap.Bounds())
			}
			testTutorialTextureGeometry(t, bitmap.Bounds(), declarations, asset)
		})
	}
}
func testTutorialTextureGeometry(t *testing.T, bounds image.Rectangle, declarations, asset string) {
	t.Helper()
	for _, test := range []struct {
		id               int
		v, height, scale float64
		rect             image.Rectangle
	}{
		{3, .5, .35, .35, image.Rect(0, 128, 128, 217)},
		{6, 0, .35, .36, image.Rect(0, 0, 128, 89)},
		{9, .36, .14, .15, image.Rect(0, 92, 128, 127)},
		{10, .85, .15, .15, image.Rect(0, 217, 128, 255)},
	} {
		t.Run(fmt.Sprintf("%s/%d", asset, test.id), func(t *testing.T) {
			p := &playState{scriptTextures: map[int]*scriptTexture{}}
			h := &playScriptHost{play: p}
			for _, call := range []struct {
				name string
				args []scripting.Value
			}{
				{"LoadTexture", []scripting.Value{test.id, strings.TrimSuffix(asset, "_SD")}},
				{"SetTextureUVs", []scripting.Value{test.id, 0, test.v, 1, test.height}},
				{"SetTextureScale", []scripting.Value{test.id, 1, test.scale}},
			} {
				if declarations != "" && call.name != "LoadTexture" {
					parts := make([]string, len(call.args))
					for i, arg := range call.args {
						parts[i] = fmt.Sprint(arg)
					}
					if !strings.Contains(declarations, call.name+"("+strings.Join(parts, ",")+")") {
						t.Fatalf("missing source call: %s %v", call.name, call.args)
					}
				}
				if _, err := h.Call(call.name, call.args); err != nil {
					t.Fatal(err)
				}
			}
			state := p.scriptTextures[test.id]
			for _, flip := range []float64{1, -1} {
				state.scaleX = flip
				rect, sx, sy := state.geometry(bounds)
				if rect != test.rect {
					t.Fatalf("crop = %v, want %v", rect, test.rect)
				}
				if math.Abs(float64(rect.Dx())*sx-flip*128) > 1e-9 || math.Abs(float64(rect.Dy())*sy-test.scale*256) > 1e-9 {
					t.Fatalf("display size = %g x %g", float64(rect.Dx())*sx, float64(rect.Dy())*sy)
				}
			}
		})
	}
	state := &scriptTexture{scaleX: .75, scaleY: .75}
	rect, sx, sy := state.geometry(image.Rect(0, 0, 128, 256))
	if rect != image.Rect(0, 0, 128, 256) || sx != .75 || sy != .75 {
		t.Fatalf("uncropped geometry = %v/%g/%g", rect, sx, sy)
	}
}
func TestTutorialTextureScreenTransform(t *testing.T) {
	state := &scriptTexture{x: 416, y: 108, scaleX: 1, scaleY: .35, u1: 0, v1: .5, u2: 1, v2: .35}
	for _, scales := range [][2]float64{{1, 1}, {2, 1}, {1, 2}, {2, 3}} {
		for _, flip := range []float64{1, -1} {
			state.scaleX = flip
			rect, transform := state.screenTransform(image.Rect(0, 0, 128, 256), scales[0], scales[1])
			x0, y0 := transform.Apply(0, 0)
			x1, y1 := transform.Apply(float64(rect.Dx()), float64(rect.Dy()))
			if math.Abs((x0+x1)/2-state.x*scales[0]) > 1e-9 || math.Abs((y0+y1)/2-state.y*scales[1]) > 1e-9 {
				t.Fatalf("screen center = %g,%g", (x0+x1)/2, (y0+y1)/2)
			}
			if math.Abs(x1-x0-flip*128*scales[0]) > 1e-9 || math.Abs(y1-y0-.35*256*scales[0]) > 1e-9 {
				t.Fatalf("screen size = %g,%g", x1-x0, y1-y0)
			}
		}
	}
}
func TestTutorialTextureZeroScaleAndPartialUVs(t *testing.T) {
	for _, test := range []struct {
		name          string
		state         scriptTexture
		rect          image.Rectangle
		width, height float64
	}{
		{"zero scale", scriptTexture{scaleX: 0, scaleY: 0}, image.Rect(0, 0, 100, 100), 0, 0},
		{"zero width", scriptTexture{scaleX: 1, scaleY: 1, v1: .29, v2: .29}, image.Rect(0, 29, 100, 58), 100, 100},
		{"zero height", scriptTexture{scaleX: 1, scaleY: 1, u1: .29, u2: .29}, image.Rect(29, 0, 58, 100), 100, 100},
		{"float32 boundary", scriptTexture{scaleX: 1, scaleY: 1, u1: .29, v1: .29, u2: .29, v2: .29}, image.Rect(29, 29, 58, 58), 100, 100},
	} {
		t.Run(test.name, func(t *testing.T) {
			rect, sx, sy := test.state.geometry(image.Rect(0, 0, 100, 100))
			if rect != test.rect || math.Abs(float64(rect.Dx())*sx-test.width) > 1e-9 || math.Abs(float64(rect.Dy())*sy-test.height) > 1e-9 {
				t.Fatalf("geometry = %v/%g/%g", rect, sx, sy)
			}
		})
	}
	p := &playState{scriptTextures: map[int]*scriptTexture{}}
	h := &playScriptHost{play: p}
	if _, err := h.Call("LoadTexture", []scripting.Value{3, "Tutorial_Image"}); err != nil {
		t.Fatal(err)
	}
	if p.scriptTextures[3].scaleX != 1 || p.scriptTextures[3].scaleY != 1 {
		t.Fatal("LoadTexture did not initialize scale to one")
	}
	if _, err := h.Call("SetTextureScale", []scripting.Value{3, 0, 0}); err != nil {
		t.Fatal(err)
	}
	_, sx, sy := p.scriptTextures[3].geometry(image.Rect(0, 0, 128, 256))
	if sx != 0 || sy != 0 {
		t.Fatalf("explicit zero scale = %g,%g", sx, sy)
	}
}
func TestFireGunUsesCurrentFacingAfterLookAt(t *testing.T) {
	for _, test := range []struct{ degrees, offsetX, offsetY float64 }{{90, -6, 24}, {45, 13, 24}, {135, -18, 21}} {
		t.Run(fmt.Sprint(test.degrees), func(t *testing.T) {
			p := &playState{x: 100, y: 100, grenades: 1, weapons: formats.WeaponCatalog{{GunType: "GRENADE", Speed: 100, Life: 1, RateOfFire: 1}}}
			h := &playScriptHost{app: &app{}, play: p}
			if _, err := h.Call("PlayerLookAt", []scripting.Value{0, 0}); err != nil {
				t.Fatal(err)
			}
			if !p.scriptHasAim || !p.flipX {
				t.Fatal("upper-left look-at did not update facing")
			}
			if _, err := h.Call("SetPlayerFacing", []scripting.Value{test.degrees}); err != nil {
				t.Fatal(err)
			}
			if _, err := h.Call("FireGun", []scripting.Value{true, false}); err != nil {
				t.Fatal(err)
			}
			if len(p.bullets) != 1 {
				t.Fatalf("secondary projectiles = %d", len(p.bullets))
			}
			shot := p.bullets[0]
			if shot.x != p.x+test.offsetX || shot.y != p.y+test.offsetY {
				t.Fatalf("secondary spawn = %g,%g; want %g,%g", shot.x, shot.y, p.x+test.offsetX, p.y+test.offsetY)
			}
			dx, dy := 96*math.Cos(test.degrees*math.Pi/180)-test.offsetX, 96*math.Sin(test.degrees*math.Pi/180)-test.offsetY
			length := math.Hypot(dx, dy)
			wantX, wantY := dx/length*100, dy/length*100
			if shot.kind != "grenade" || math.Abs(shot.vx-wantX) > 1e-8 || math.Abs(shot.vy-wantY) > 1e-8 {
				t.Fatalf("secondary velocity = %g,%g; want %g,%g", shot.vx, shot.vy, wantX, wantY)
			}
			if shot.vx*(p.scriptAimX-shot.x)+shot.vy*(p.scriptAimY-shot.y) >= 0 {
				t.Fatal("secondary velocity points toward the stale upper-left look-at target")
			}
		})
	}
}
