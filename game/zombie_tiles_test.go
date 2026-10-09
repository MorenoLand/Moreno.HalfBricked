package game

import (
	"math"
	"testing"

	"github.com/MorenoLand/Moreno.HalfBricked/engine/formats"
	"github.com/MorenoLand/Moreno.HalfBricked/engine/weapons"
)

func tileRig(t *testing.T, wall func(x, y int) bool) *playState {
	p := wallTestPlay(t, wall)
	rng := weapons.NewNativeRNG()
	p.rng = &rng
	p.alertNoise = 70 // after the first shot every zombie is alerted (FUN_000947e0)
	return p
}

func spawnRisen(p *playState, name string, x, y float64) *zombieState {
	p.spawnZombieAt(formats.SpawnType{Name: name, Chance: 1, Speed: formats.Vec2{X: 90, Y: 90}, Strength: 100, Size: formats.Vec2{X: 29, Y: 29}, TurnSpeed: 12, Texture: "cavezombie", DeviateAmount: 17, DeviateCycleSpeed: 45, AlertRadius: 250}, formats.Vec2{X: x, Y: y})
	z := &p.zombies[len(p.zombies)-1]
	z.native.ai.riseMS = zombieRiseDoneMS - 1 // skip the 2 s spawn wait
	return z
}

func insideWall(p *playState, z *zombieState) bool {
	return playerCollisionBlocks(p.collisionValue(int(z.x)/p.tileSize, int(z.y)/p.tileSize))
}

// Native zombies have no path search, stuck timer or sidestep: they walk along
// their heading and FUN_000be3c0 pushes them out of blocking tiles. Around a
// convex corner the circle push slides them past the corner toward the player.
func TestZombieRoundsConvexWallCornerLikeTheOriginal(t *testing.T) {
	// A wall column (rows 0..4) ends at y=160; the player is left of and below its end.
	p := tileRig(t, func(x, y int) bool { return x == 8 && y < 5 })
	p.x, p.y = 6*32+16, 6*32+16
	z := spawnRisen(p, "zombie", 9*32+16, 4*32+16)
	closest := math.Inf(1)
	for frame := 0; frame < 60*10; frame++ {
		p.updateZombies()
		if insideWall(p, z) {
			t.Fatalf("zombie entered a wall tile at %.1f,%.1f on frame %d", z.x, z.y, frame)
		}
		closest = math.Min(closest, math.Hypot(z.x-p.x, z.y-p.y))
	}
	if closest > 40 {
		t.Fatalf("zombie never got round the corner (closest %.0f px)", closest)
	}
}

// Head-on into a wall the original keeps pushing against it (no unstick): the
// body is held outside the tiles at 0.3 * width from the face.
func TestZombieHeldOutsideWallsHeadOn(t *testing.T) {
	p := tileRig(t, func(x, y int) bool { return x == 8 })
	p.x, p.y = 4*32+16, 4*32+16
	z := spawnRisen(p, "zombie", 11*32+16, 4*32+16)
	for frame := 0; frame < 60*10; frame++ {
		p.updateZombies()
	}
	face := 9.0 * 32
	if z.x < face+zombieCollisionRadius(*z)-1.5 {
		t.Fatalf("zombie at x=%.1f is inside the wall face %.0f + radius %.1f", z.x, face, zombieCollisionRadius(*z))
	}
	if math.Hypot(z.x-p.x, z.y-p.y) < 100 {
		t.Fatal("a zombie cannot cross a full-height wall")
	}
}

// Smart zombies (FUN_0009c168) are the only class with a path: this one walks
// through the gap of a wall.
func TestSmartZombieWalksAroundWall(t *testing.T) {
	p := tileRig(t, func(x, y int) bool { return x == 8 && y < 7 })
	p.x, p.y = 13*32+16, 3*32+16
	z := spawnRisen(p, "smart_zombie", 3*32+16, 3*32+16)
	closest := math.Inf(1)
	for frame := 0; frame < 60*30; frame++ {
		p.updateZombies()
		closest = math.Min(closest, math.Hypot(z.x-p.x, z.y-p.y))
	}
	if closest > 40 {
		t.Fatalf("smart zombie never reached the player (closest %.0f px)", closest)
	}
}

func TestNativeTilePushLeavesCircleOutsideBlocks(t *testing.T) {
	p := tileRig(t, func(x, y int) bool { return x == 4 && y == 4 })
	// Penetrating the left face of the tile at x=128..160.
	px, py := p.nativeTilePush(128-10, 144, 15)
	if px >= 0 || math.Abs(px) < 4.9 || math.Abs(px) > 5.1 || py != 0 {
		t.Fatalf("push %v,%v, want about -5,0", px, py)
	}
	if px, py := p.nativeTilePush(100, 144, 15); px != 0 || py != 0 {
		t.Fatalf("free body pushed %v,%v", px, py)
	}
	// Value 2 blocks too (the zombie call passes 1 as last argument).
	p.world.Level.Layers[formats.LayerC][4*16+4] = 1
	if px, _ := p.nativeTilePush(128-10, 144, 15); px >= 0 {
		t.Fatal("collision value 2 must block zombies")
	}
}
