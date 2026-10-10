package game

import (
	"github.com/MorenoLand/Moreno.HalfBricked/engine/formats"
	"math"
	"testing"
)

// run steps the zombie frames and returns the closest approach to the player and
// the frame it first came within reach (or -1).
func unstickRun(p *playState, z *zombieState, frames int) (closest float64, arrived int) {
	closest, arrived = math.Inf(1), -1
	for frame := 0; frame < frames; frame++ {
		p.updateZombies()
		d := math.Hypot(z.x-p.x, z.y-p.y)
		closest = math.Min(closest, d)
		if arrived < 0 && d < 60 {
			arrived = frame
		}
	}
	return
}

// PORT ADDITION: a zombie wedged head-on at a wall with the player right across
// escapes round it; with zombieSmartExtras off it stays pinned (original).
func TestUnstickFreesZombieWedgedAtWall(t *testing.T) {
	setup := func() (*playState, *zombieState) {
		p := tileRig(t, func(x, y int) bool { return x == 8 && y < 6 })
		p.x, p.y = 9*32+16, 3*32+16
		z := spawnRisen(p, "zombie", 7*32+16, 3*32+16)
		return p, z
	}
	defer func(old bool) { zombieSmartExtras = old }(zombieSmartExtras)
	zombieSmartExtras = true
	p, z := setup()
	closest, arrived := unstickRun(p, z, 60*8)
	if arrived < 0 || arrived > 60*6 {
		t.Fatalf("zombie did not get round the wall (closest %.0f, arrived frame %d)", closest, arrived)
	}
	zombieSmartExtras = false
	p, z = setup()
	if closest, arrived = unstickRun(p, z, 60*8); arrived >= 0 {
		t.Fatalf("faithful model unexpectedly escaped (closest %.0f, frame %d): the test no longer shows the wedge", closest, arrived)
	}
}

// PORT ADDITION: no wall contact, no change: identical positions frame by frame.
func TestUnstickLeavesOpenFieldChaseUntouched(t *testing.T) {
	defer func(old bool) { zombieSmartExtras = old }(zombieSmartExtras)
	trace := func(enabled bool) [][2]float64 {
		zombieSmartExtras = enabled
		p := tileRig(t, func(x, y int) bool { return false })
		p.x, p.y = 12*32+16, 5*32+16
		var zs []*zombieState
		for i := 0; i < 4; i++ {
			zs = append(zs, spawnRisen(p, "zombie", float64(2*32+16+i*20), float64(3*32+16+i*30)))
		}
		var out [][2]float64
		for frame := 0; frame < 60*5; frame++ {
			p.updateZombies()
			for index := range p.zombies {
				out = append(out, [2]float64{p.zombies[index].x, p.zombies[index].y})
			}
		}
		_ = zs
		return out
	}
	a, b := trace(false), trace(true)
	if len(a) != len(b) {
		t.Fatal("trace length")
	}
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("open-field trajectory changed at sample %d: %v vs %v", i, a[i], b[i])
		}
	}
}

// PORT ADDITION: walls are never entered and a trapped-looking zombie still ends
// up in reach of the player so the wave can be finished.
func TestUnstickNeverEntersWallsAndStaysKillable(t *testing.T) {
	p := tileRig(t, func(x, y int) bool { return (x == 8 && y < 6) || (y == 6 && x >= 5 && x <= 8) })
	p.x, p.y = 11*32+16, 3*32+16
	z := spawnRisen(p, "zombie", 7*32+16, 5*32+16)
	closest, arrived := math.Inf(1), -1
	for frame := 0; frame < 60*20; frame++ {
		p.updateZombies()
		if insideWall(p, z) {
			t.Fatalf("entered wall at %.1f,%.1f", z.x, z.y)
		}
		d := math.Hypot(z.x-p.x, z.y-p.y)
		closest = math.Min(closest, d)
		if arrived < 0 && d < 60 {
			arrived = frame
		}
	}
	if arrived < 0 {
		t.Fatalf("zombie never reached the player (closest %.0f)", closest)
	}
	z.health = 0
	p.updateZombies()
}

// PORT ADDITION: the rex (rexSteer) gets the same unstick as wave zombies.
func TestUnstickKeepsTheRexMovingAroundWalls(t *testing.T) {
	setup := func() (*playState, *zombieState) {
		p := tileRig(t, func(x, y int) bool { return x == 8 && y < 6 })
		p.scriptEntities = map[int]*scriptEntity{rexTestID: {id: rexTestID, kind: "zombie", entityType: "boss_rex"}}
		p.zombies = append(p.zombies, zombieState{x: 7*32 + 16, y: 3*32 + 16, health: 25000, size: formats.Vec2{X: 64, Y: 64}, scriptID: rexTestID, alpha: 1, speed: 170})
		p.x, p.y = 9*32+16, 3*32+16
		return p, &p.zombies[0]
	}
	defer func(old bool) { zombieSmartExtras = old }(zombieSmartExtras)
	zombieSmartExtras = true
	p, z := setup()
	closest, arrived := unstickRun(p, z, 60*8)
	if arrived < 0 || insideWall(p, z) {
		t.Fatalf("rex stayed wedged (closest %.0f)", closest)
	}
}
