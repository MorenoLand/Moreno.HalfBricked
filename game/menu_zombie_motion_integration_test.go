package game

import (
	"github.com/MorenoLand/Moreno.HalfBricked/engine/menumotion"
	"math"
	"testing"
)

func TestMainMenuNativeMotionKeepsCompatibilityAnchors(t *testing.T) {
	a := &app{}
	a.updateMenuZombieMotion(float32(1.0 / 60.0))
	anchors := [][2]float32{{76, 105}, {210, 194}, {76, 274}, {377, 264}}
	if len(a.menuZombies) != len(anchors) || a.rng != nil {
		t.Fatalf("four menu groups must not consume gameplay RNG: %+v", a.menuZombies)
	}
	for i, m := range a.menuZombies {
		if m.BaseX != anchors[i][0] || m.BaseY != anchors[i][1] || m.Phase < 0 || m.Phase >= 3 || m.State != menumotion.NativeMenuZombieEntering {
			t.Fatalf("group %d: %+v", i, m)
		}
		for j := 0; j < i; j++ {
			if m.Phase == a.menuZombies[j].Phase {
				t.Fatalf("groups %d/%d share a phase", i, j)
			}
		}
	}
	for range 180 {
		a.updateMenuZombieMotion(float32(1.0 / 60.0))
	}
	for i, m := range a.menuZombies {
		if m.State != menumotion.NativeMenuZombieIdle || m.Progress != 1 || m.Scale != m.BaseScale || math.Abs(math.Hypot(float64(m.X-m.BaseX), float64(m.Y-m.BaseY))-float64(m.BaseScale*10)) > .00005 {
			t.Fatalf("idle group %d: %+v", i, m)
		}
	}
}
func TestMainMenuGroupTransformRetainsChildGeometry(t *testing.T) {
	m := menumotion.NewNativeMenuZombieMotion(210, 194, 0, .75, 0)
	m.X, m.Y, m.Scale, m.RotationDegrees = 217, 189, .375, 8
	for _, child := range [][7]float64{{.325, .325, 128, 128, 0, -8, 0}, {.8, .8, 70, 35, 0, 0, .4}, {.4, .4, 128, 32, 0, 0, .4}, {-.3, .4, 32, 32, -45, 6, math.Pi / 2}} {
		op := menuGroupImageOptions(m, child[0], child[1], child[2], child[3], child[4], child[5], child[6])
		x, y := op.GeoM.Apply(child[2], child[3])
		angle := float64(m.RotationDegrees) * math.Pi / 180
		angle = -angle // native rotation.z is counter-clockwise positive
		wantX := float64(m.X) + .5*(math.Cos(angle)*child[4]-math.Sin(angle)*child[5])
		wantY := float64(m.Y) + .5*(math.Sin(angle)*child[4]+math.Cos(angle)*child[5])
		if math.Abs(x-wantX) > 1e-9 || math.Abs(y-wantY) > 1e-9 {
			t.Fatalf("child center %v: (%v,%v), want (%v,%v)", child, x, y, wantX, wantY)
		}
		x1, y1 := op.GeoM.Apply(child[2]+1, child[3])
		if math.Abs(math.Hypot(x1-x, y1-y)-.5*math.Abs(child[0])) > 1e-9 {
			t.Fatalf("child scale applied more than once: %v", child)
		}
	}
}
func TestMainMenuClickFreezesSelectedGroupAndLeavesOthers(t *testing.T) {
	a := &app{}
	for range 17 {
		a.updateMenuZombieMotion(float32(1.0 / 60.0))
	}
	m := a.menuZombies[1]
	if err := a.beginMenuClick(1); err != nil {
		t.Fatal(err)
	}
	if a.menuClick.x != float64(m.X) || a.menuClick.y != float64(m.Y) {
		t.Fatal("click detached from current orbit center")
	}
	for range 20 {
		a.updateMenuZombieMotion(float32(1.0 / 60.0))
	}
	for i, got := range a.menuZombies {
		if i == 1 {
			if got.State != menumotion.NativeMenuZombieClicked || got.X != m.X || got.Y != m.Y || got.Phase != m.Phase || got.RotationDegrees != m.RotationDegrees {
				t.Fatalf("clicked group moved: %+v", got)
			}
		} else if got.State != menumotion.NativeMenuZombieHidden || got.Visible || got.Scale != 0 {
			t.Fatalf("group %d did not finish native exit: %+v", i, got)
		}
	}
}
func TestMainMenuReentryAndTransformedHit(t *testing.T) {
	a := &app{}
	for range 17 {
		a.updateMenuZombieMotion(float32(1.0 / 60.0))
	}
	m := &a.menuZombies[1]
	m.X, m.Y, m.Scale, m.RotationDegrees = 225, 180, m.BaseScale*.5, 8
	op := menuGroupImageOptions(*m, 1, 1, 0, 0, 0, 0, mainMenuButtons[1].angle)
	x, y := op.GeoM.Apply(45, 0)
	if got := a.mainMenuHit(int(math.Round(x)), int(math.Round(y))); got != 1 {
		t.Fatalf("moving button hit=%d", got)
	}
	m.Visible = false
	if got := a.mainMenuHit(225, 180); got == 1 {
		t.Fatal("hidden group is still clickable")
	}
	a.page = 3
	a.updateMenuZombieMotion(float32(1.0 / 60.0))
	a.page = 0
	a.updateMenuZombieMotion(float32(1.0 / 60.0))
	if !a.menuMotionActive || !a.menuZombies[1].Visible || a.menuZombies[1].State != menumotion.NativeMenuZombieEntering || a.menuZombies[1].Progress >= 1 {
		t.Fatal("return did not replay entry envelope")
	}
}
