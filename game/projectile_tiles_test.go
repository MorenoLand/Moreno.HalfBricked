package game

import (
	"encoding/json"
	"math"
	"os"
	"testing"

	"github.com/MorenoLand/Moreno.HalfBricked/engine/formats"
	"github.com/MorenoLand/Moreno.HalfBricked/engine/viewer"
	"github.com/MorenoLand/Moreno.HalfBricked/engine/weapons"
)

// realLevelPlay builds a playState over the real collision layer of a cached level
// (bin/data-cache, or the old bin/web/data/data-cache), or skips.
func realLevelPlay(t *testing.T, name string) *playState {
	t.Helper()
	var data []byte
	for _, dir := range []string{"../bin/data-cache/levels/", "../bin/web/data/data-cache/levels/"} {
		if b, err := os.ReadFile(dir + name + ".json"); err == nil {
			data = b
			break
		}
	}
	if data == nil {
		t.Skip("original level data not installed")
	}
	var raw struct {
		Width, Height int
		Layers        map[string][]uint32
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatal(err)
	}
	return &playState{
		world:     &viewer.Viewer{Level: formats.Level{Width: raw.Width, Height: raw.Height, Layers: map[formats.LayerKind][]uint32{formats.LayerC: raw.Layers["c"]}}, Zoom: 1},
		tileSize:  32,
		health:    1,
		maxHealth: 1,
	}
}

// centre returns the pixel centre of a tile.
func tileCentre(tx, ty int) (float64, float64) { return (float64(tx) + .5) * 32, (float64(ty) + .5) * 32 }

// World0Level1 (53 x 33): collision value 1 is the cliff/rock mass (e.g. tile
// 10,5), value 2 is the void around the arena and the 5 x 5 pits inside it (tile
// 13,19), 3..16 are spawn markers sitting on open ground (tile 21,6 = 4), and 0
// is open sand (tile 20,14).
func TestProjectileTileValuesOnRealLevel(t *testing.T) {
	p := realLevelPlay(t, "World0Level1")
	for _, c := range []struct {
		name   string
		tx, ty int
		value  uint32
		bullet bool
		walk   bool
	}{
		{"cliff", 10, 5, 1, true, true},
		{"pit", 13, 19, 2, false, true},
		{"void", 2, 5, 2, false, true},
		{"spawn marker", 21, 6, 4, false, false},
		{"open sand", 20, 14, 0, false, false},
	} {
		if got := p.collisionValue(c.tx, c.ty); got != c.value {
			t.Fatalf("%s tile value = %d, want %d (level data changed?)", c.name, got, c.value)
		}
		x, y := tileCentre(c.tx, c.ty)
		if got := p.projectileBlockedAt(x, y); got != c.bullet {
			t.Fatalf("%s: projectile blocked = %v, want %v", c.name, got, c.bullet)
		}
		if got := p.isSolid(x, y); got != c.walk {
			t.Fatalf("%s: walking blocked = %v, want %v (walking rule must be unchanged)", c.name, got, c.walk)
		}
	}
	// FUN_000be2c4 reports -1 outside the level, which is not a wall.
	for _, pt := range [][2]float64{{-40, 100}, {100, -40}, {53 * 32, 100}, {100, 33 * 32}} {
		if p.projectileBlockedAt(pt[0], pt[1]) {
			t.Fatalf("outside the level at %v must not stop a projectile", pt)
		}
	}
}

func bulletRun(p *playState, x, y, vx, vy float64, frames int) (bullet, bool) {
	p.bullets = []bullet{{x: x, y: y, vx: vx, vy: vy, life: 10}}
	for frame := 0; frame < frames && len(p.bullets) > 0; frame++ {
		p.updateBulletsAndKills()
	}
	if len(p.bullets) == 0 {
		return bullet{}, false
	}
	return p.bullets[0], true
}

// A bullet flies straight across the 5 x 5 pit (value 2) but is stopped by the
// cliff (value 1) of the same level.
func TestBulletCrossesPitAndStopsAtCliff(t *testing.T) {
	p := realLevelPlay(t, "World0Level1")
	x0, y := tileCentre(9, 19)
	b, alive := bulletRun(p, x0, y, 600, 0, 30) // 300 px: across tiles 10..16 (pit 11..15)
	if !alive || b.x < 16*32 {
		t.Fatalf("a bullet must cross the pit tiles 11..15, got alive=%v x=%.0f", alive, b.x)
	}
	// Fire up into the cliff from open ground below it (tile 20,12 -> 20,10 is 1).
	cx, cy := tileCentre(20, 12)
	if p.collisionValue(20, 10) != 1 {
		t.Fatalf("tile 20,10 = %d, want the cliff", p.collisionValue(20, 10))
	}
	if _, alive := bulletRun(p, cx, cy, 0, -300, 60); alive {
		t.Fatal("a bullet must be removed by a value-1 tile")
	}
}

// World0Level2 has the bone piles that were reported as bullet blockers: the tiles
// under the bones prop at (558,294) are open ground, a bullet flies through them.
func TestBulletPassesBonePropTilesOnRealLevel(t *testing.T) {
	p := realLevelPlay(t, "World0Level2")
	tx, ty := 558/32, 294/32
	for dx := -2; dx <= 2; dx++ {
		if v := p.collisionValue(tx+dx, ty); v == 1 {
			t.Fatalf("tile %d,%d under the bones prop is a bullet wall", tx+dx, ty)
		}
	}
	x, y := tileCentre(tx-2, ty)
	if _, alive := bulletRun(p, x, y, 300, 0, 30); !alive {
		t.Fatal("a bullet must fly through the bones prop")
	}
}

// The thrown bomb bounces on value-1 tiles only (FUN_000be3c0 flag 0).
func TestThrownBombIgnoresPitButBouncesOffCliff(t *testing.T) {
	p := realLevelPlay(t, "World0Level1")
	x, y := tileCentre(10, 19)
	p.thrown = []thrownBomb{{x: x, y: y, lift: 20, fall: -70, dirX: 1, speed: 300, life: 4, size: grenadeSize, bounce: true, parentLife: 4, parentSpeed: 300}}
	for frame := 0; frame < 30; frame++ {
		p.updateThrown()
	}
	if len(p.thrown) != 1 || p.thrown[0].dirX <= 0 || p.thrown[0].x < 12*32 {
		t.Fatalf("a bomb must roll over the pit: %+v", p.thrown)
	}
	x, y = tileCentre(20, 12)
	p.thrown = []thrownBomb{{x: x, y: y, lift: 20, fall: -70, dirY: -1, speed: 300, life: 4, size: grenadeSize, bounce: true, parentLife: 4, parentSpeed: 300}}
	for frame := 0; frame < 20 && len(p.thrown) == 1 && p.thrown[0].dirY < 0; frame++ {
		p.updateThrown()
	}
	if len(p.thrown) != 1 || p.thrown[0].dirY <= 0 || p.thrown[0].speed >= 300 {
		t.Fatalf("a bomb must bounce back off the cliff: %+v", p.thrown)
	}
}

// The flame stops against a value-1 wall instead of dying, and passes over pits.
func TestFlameStopsAtCliffAndCrossesPit(t *testing.T) {
	p := realLevelPlay(t, "World0Level1")
	flame := func() *weapons.NativeWeaponProjectile {
		return &weapons.NativeWeaponProjectile{EntityType: 0x16, Width: 24, Life: 10}
	}
	x, y := tileCentre(20, 12)
	p.bullets = []bullet{{x: x, y: y, vy: -300, life: 10, projectile: flame()}}
	for frame := 0; frame < 40; frame++ {
		p.updateBulletsAndKills()
	}
	if len(p.bullets) != 1 || p.bullets[0].vx != 0 || p.bullets[0].vy != 0 || p.bullets[0].y < 11*32-math.Pi {
		t.Fatalf("flame should stop at the cliff face and stay: %+v", p.bullets)
	}
	x, y = tileCentre(9, 19)
	p.bullets = []bullet{{x: x, y: y, vx: 300, life: 10, projectile: flame()}}
	for frame := 0; frame < 30; frame++ {
		p.updateBulletsAndKills()
	}
	if len(p.bullets) != 1 || p.bullets[0].x < 14*32 {
		t.Fatalf("flame should cross the pit: %+v", p.bullets)
	}
}
