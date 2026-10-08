package game

import (
	"math"
	"os"
	"testing"

	"github.com/MorenoLand/Moreno.HalfBricked/engine/formats"
	"github.com/MorenoLand/Moreno.HalfBricked/engine/menumotion"
)

func loadMainScreenForTest(t *testing.T) *formats.UIScreen {
	t.Helper()
	data, err := os.ReadFile("bin/data-cache/source/Common0/UserInterface/screens/MainScreen.uiscreen")
	if err != nil {
		t.Skip("MainScreen.uiscreen not cached")
	}
	screen, err := formats.ParseUIScreen(data)
	if err != nil {
		t.Fatal(err)
	}
	return screen
}

// The hands, board and body of every zombie come from MainScreen.uiscreen, with
// no per-zombie tables in the port.
func TestMenuGroupLayoutsComeFromMainScreen(t *testing.T) {
	screen := loadMainScreenForTest(t)
	for _, tc := range []struct {
		group                string
		art                  int
		leftFlip, rightFlip  bool
		leftW, leftH         float64
		boardRot, boardW     float64
		buttonW, buttonH, by float64
	}{
		{"OptionsZombie", 3, true, false, 33, 32, -6, 101, 92, 62, 26},
		{"PlayZombie", 2, true, false, 28, 32, -6, 101, 92, 62, 26},
		{"QuitZombie", 2, true, false, 28, 32, -6, 101, 92, 62, 26},
		{"StatsZombie", 1, false, true, 34, 44, -6, 101, 92, 62, 26},
	} {
		layout := parseMenuGroupLayout(screen, tc.group)
		if layout == nil || !layout.hands {
			t.Fatalf("%s: no layout", tc.group)
		}
		if layout.back.art != tc.art || layout.back.w != 116 || layout.back.h != 135 || layout.back.x != 5 || layout.back.y != 10 {
			t.Fatalf("%s body %+v", tc.group, layout.back)
		}
		if layout.left.flipX != tc.leftFlip || layout.right.flipX != tc.rightFlip || layout.left.flipY || layout.right.flipY {
			t.Fatalf("%s flips left=%v right=%v", tc.group, layout.left.flipX, layout.right.flipX)
		}
		if layout.left.w != tc.leftW || layout.left.h != tc.leftH {
			t.Fatalf("%s left hand size %v x %v", tc.group, layout.left.w, layout.left.h)
		}
		if layout.screen.rotation != tc.boardRot || layout.screen.w != tc.boardW || layout.screen.h != 67 || layout.screen.x != 1 || layout.screen.y != 27 {
			t.Fatalf("%s board %+v", tc.group, layout.screen)
		}
		if layout.button.w != tc.buttonW || layout.button.h != tc.buttonH || layout.button.y != tc.by {
			t.Fatalf("%s button %+v", tc.group, layout.button)
		}
	}
	if parseMenuGroupLayout(screen, "NoSuchZombie") != nil {
		t.Fatal("unknown group must not produce a layout")
	}
}

// A child's image is mirrored first (negative texCoordSize), then rotated by the
// negated rotation.z (positive is counter-clockwise on screen), then scaled and
// moved with the group.
func TestMenuGroupChildTransformOrder(t *testing.T) {
	layout := &menuGroupLayout{screen: menuGroupChild{}}
	motion := menumotion.NewNativeMenuZombieMotion(100, 80, 0, 1, 0)
	motion.Scale = 2
	// Unflipped, rotation +90 (counter-clockwise): the image's right edge goes up.
	child := menuGroupChild{x: 10, y: 4, w: 20, h: 10, rotation: 90}
	op := menuGroupChildOptions(layout, motion, child, 40, 20, 20, 10, 1, 1)
	x, y := op.GeoM.Apply(40, 10)
	if math.Abs(x-(100+2*10)) > 1e-9 || math.Abs(y-(80+2*(4-10))) > 1e-9 {
		t.Fatalf("unflipped right edge at %v,%v", x, y)
	}
	// Flipped, rotation +90: the image's right edge is first mirrored to the left
	// edge, then the counter-clockwise quarter turn sends the left edge down.
	child.flipX = true
	op = menuGroupChildOptions(layout, motion, child, 40, 20, 20, 10, 1, 1)
	x, y = op.GeoM.Apply(40, 10)
	if math.Abs(x-(100+2*10)) > 1e-9 || math.Abs(y-(80+2*(4+10))) > 1e-9 {
		t.Fatalf("flipped right edge at %v,%v", x, y)
	}
	// The group's own rotation is also negated: +90 sends the child offset (10,0)
	// from the group centre to (0,-10) scaled by 2.
	motion.RotationDegrees = 90
	flat := menuGroupChild{x: 10, w: 4, h: 4}
	op = menuGroupChildOptions(layout, motion, flat, 4, 4, 2, 2, 1, 1)
	x, y = op.GeoM.Apply(2, 2)
	if math.Abs(x-100) > 1e-9 || math.Abs(y-(80-20)) > 1e-9 {
		t.Fatalf("group rotation moved the child to %v,%v", x, y)
	}
}

func TestMenuGroupHitUsesTheButtonComponent(t *testing.T) {
	layout := &menuGroupLayout{screen: menuGroupChild{x: 1, y: 27}, button: menuGroupChild{x: 0, y: 26, w: 92, h: 62}}
	motion := menumotion.NewNativeMenuZombieMotion(200, 150, 0, .75, 0)
	originX, originY := menuGroupOrigin(layout, motion)
	if math.Abs(originX-(200-.75)) > 1e-9 || math.Abs(originY-(150-27*.75)) > 1e-9 {
		t.Fatalf("group origin %v,%v", originX, originY)
	}
	inside := func(dx, dy float64) bool { return menuGroupHit(layout, motion, originX+dx*.75, originY+dy*.75) }
	if !inside(45, 26) || !inside(-45, 26+30) || inside(47, 26) || inside(0, 26+32) {
		t.Fatal("button rectangle edges")
	}
	motion.Visible = false
	if inside(0, 26) {
		t.Fatal("hidden group is still clickable")
	}
}
