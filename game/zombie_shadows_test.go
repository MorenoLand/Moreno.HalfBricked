package game

import (
	"image"
	"math"
	"strings"
	"testing"

	"github.com/MorenoLand/Moreno.HalfBricked/engine/formats"
	"github.com/MorenoLand/Moreno.HalfBricked/engine/viewer"
	"github.com/hajimehoshi/ebiten/v2"
)

func TestZombieShadowNativeGeometry(t *testing.T) {
	world := &viewer.Viewer{CameraX: 100, CameraY: 200, Zoom: 2, ViewportX: 10, ViewportY: 20}
	zombie := zombieState{x: 150, y: 250, size: formats.Vec2{X: 48, Y: 60}, alpha: 1}
	shadow, visible := zombieShadowGeometry(zombie, world, image.Rect(0, 0, logicalWidth, logicalHeight))
	if !visible || shadow.x != 110 || shadow.y != 120 || shadow.alpha != 127 || shadow.width != float64(math.Float32frombits(0x4219999a))*2 || shadow.height != float64(math.Float32frombits(0x41cc985f))*2 {
		t.Fatalf("native geometry = %+v, visible %t", shadow, visible)
	}
	zombie.alpha = .5
	shadow, visible = zombieShadowGeometry(zombie, world, image.Rect(0, 0, logicalWidth, logicalHeight))
	if !visible || shadow.alpha != 63 {
		t.Fatalf("half opacity = %+v, visible %t", shadow, visible)
	}
}
func TestZombieShadowTextureAndCap(t *testing.T) {
	body := ebiten.NewImage(1, 1)
	defer body.Dispose()
	a := &app{images: map[string]*ebiten.Image{strings.ToLower(commonSDTexture("cavezombie")): body}, play: &playState{world: &viewer.Viewer{Zoom: 1}, zombies: make([]zombieState, 513)}}
	for i := range a.play.zombies {
		a.play.zombies[i] = zombieState{x: 100, y: 100, alpha: 1}
	}
	if got := len(a.zombieShadows()); got != 511 {
		t.Fatalf("without player = %d, want 511", got)
	}
	a.images["common0/textures/characters/barryidle_sd"] = body
	if got := len(a.zombieShadows()); got != 510 {
		t.Fatalf("with player = %d, want 510", got)
	}
	a.play.zombies = a.play.zombies[:3]
	a.play.zombies[0].texture = "missing"
	a.play.zombies[1].alpha = 0
	if got := len(a.zombieShadows()); got != 1 {
		t.Fatalf("texture/alpha suppression = %d, want 1", got)
	}
}
func TestZombieShadowAnchoredCulling(t *testing.T) {
	world := &viewer.Viewer{Zoom: 1}
	for _, test := range []struct {
		name        string
		x, y, alpha float64
		inner       image.Rectangle
		visible     bool
	}{
		{"body above screen", 100, -8, 1, image.Rect(0, 0, 480, 320), false},
		{"body crosses bottom", 100, 340, 1, image.Rect(0, 0, 480, 320), true},
		{"body touches left", -24, 100, 1, image.Rect(0, 0, 480, 320), false},
		{"body crosses left", -23, 100, 1, image.Rect(0, 0, 480, 320), true},
		{"outside inner", 40, 100, 1, image.Rect(80, 0, 480, 320), false},
		{"crosses inner", 60, 100, 1, image.Rect(80, 0, 480, 320), true},
		{"transparent", 100, 100, 0, image.Rect(0, 0, 480, 320), false},
		{"quantized transparent", 100, 100, 1.0 / 255, image.Rect(0, 0, 480, 320), false},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, visible := zombieShadowGeometry(zombieState{x: test.x, y: test.y, alpha: test.alpha, size: formats.Vec2{X: 48, Y: 48}}, world, test.inner)
			if visible != test.visible {
				t.Fatalf("visible = %t, want %t", visible, test.visible)
			}
		})
	}
}
