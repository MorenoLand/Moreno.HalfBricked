package game

import (
	"testing"

	"github.com/MorenoLand/Moreno.HalfBricked/engine/formats"
)

func carRig() (*playState, *zombieState) {
	p := modelRig()
	p.zombieWeapons = nil
	entry := formats.SpawnType{Name: "boss_gangster", Speed: formats.Vec2{X: 300}, Strength: 15000, Size: formats.Vec2{X: 96}, TurnSpeed: 15, Texture: "gangstercar"}
	p.spawnZombieAt(entry, formats.Vec2{X: 400, Y: 300})
	return p, &p.zombies[0]
}

// The car spawns off the left edge on the spawn point's row, is 0.6 as tall as it is wide, drives right, never
// follows the player's row and turns round at (levelWidth + 6) * 32 (FUN_000b732c / FUN_000b6b90 / FUN_000b7264).
func TestGangsterCarDrivesItsLaneAndTurnsRound(t *testing.T) {
	p, z := carRig()
	p.x, p.y = 900, 120
	if z.x != carStartX || z.y != 300 {
		t.Fatalf("car starts at %.0f,%.0f, want -192,300", z.x, z.y)
	}
	const dt = 1.0 / 60.0
	startedMoving := false
	turned := false
	right := (float64(p.world.Level.Width) + carRightMarginTiles) * carTileSize
	for frame := 0; frame < 60*120 && !turned; frame++ {
		before := z.x
		p.updateZombies()
		if z.x > before {
			startedMoving = true
		}
		if z.y != 300 {
			t.Fatalf("the car left its lane: y %.1f", z.y)
		}
		if z.native.car.dir < 0 {
			turned = true
		}
	}
	if !startedMoving || !turned {
		t.Fatalf("moving %v turned %v at x %.0f (right edge %.0f)", startedMoving, turned, z.x, right)
	}
	if z.size.Y < z.size.X*carHeightFactor-.01 || z.size.Y > z.size.X*carHeightFactor+.01 {
		t.Fatalf("car %.1f x %.1f, want height 0.6 of the width", z.size.X, z.size.Y)
	}
	_ = dt
}

// Touching the car hurts at 3 * 0.05 per second and empties its gun.
func TestGangsterCarContactHurtsAndEmptiesTheGun(t *testing.T) {
	p, z := carRig()
	z.native.ai.state = 1
	z.x, z.y = 500, 300
	z.native.ai.gun.rounds = 9
	p.x, p.y = z.x, z.y
	p.health = 1
	prey := playerTarget{index: 0, x: p.x, y: p.y}
	p.carContact(z, prey, 1.0/60.0)
	if p.health >= 1 {
		t.Fatalf("no contact damage: health %v", p.health)
	}
	if z.native.ai.gun.rounds != 0 {
		t.Fatalf("gun not emptied: %d rounds", z.native.ai.gun.rounds)
	}
}

// The car sprite is mirrored from its drive direction, never from where the player stands.
func TestGangsterCarFlipFollowsTheDriveDirection(t *testing.T) {
	p, z := carRig()
	z.native.ai.state = 1
	z.x = 300
	for _, playerX := range []float64{50, 900} {
		p.x, p.y = playerX, 100
		p.updateZombies()
		if !z.flipX {
			t.Fatalf("driving right with the player at %.0f: the car must be mirrored", playerX)
		}
	}
	z.native.car.dir = -1
	p.updateZombies()
	if z.flipX {
		t.Fatal("driving left: the car art already faces left")
	}
}
